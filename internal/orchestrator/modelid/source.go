// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelid

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrInvalidID     = errors.New("not a valid model id")
	ErrInvalidSource = errors.New("not a valid model source")
)

// Mirrors the models-downloader: the quantization a key defaults to when it names only a
// repository, the host a file URL must be served by, and the shape of a Hub name. Keep
// them in step with common/gguf_naming.py and hugging_face/hf_downloader.py.
const (
	defaultQuantization = "Q4_0"
	hubHost             = "huggingface.co"
)

var (
	hubName   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$`)
	urlScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
)

// Source is a request to fetch a model no models-list.yaml entry declares. It is
// deliberately not an ID: the identity exists only once a file has landed, which is why
// the handler reports it on the download event.
//
// Two forms, the same two the handler accepts:
//
//	https://huggingface.co/<owner>/<repo>/resolve/<revision>/<file>.gguf
//	[<framework>:]<owner>/<repo>[:<quantization>[:<mmproj quantization>]]
//
// What is checked here is what can be checked without the network, so that a typo costs
// a bad request rather than a container start and a Hub round trip. The handler stays the
// authority: everything this accepts, it validates again, and it looks at more.
type Source struct {
	spec                  string
	isURL                 bool
	repoID                string
	quantization          string
	quantizationDefaulted bool
	mmprojQuantization    string
	mmprojURL             string
}

// ParseSource reads a download source. mmprojURL is the multimodal projection file from
// the request body, which belongs to the URL form only: a key carries its mmproj
// quantization as a fourth field instead.
func ParseSource(spec, mmprojURL string) (Source, error) {
	if strings.TrimSpace(spec) != spec || spec == "" {
		return Source{}, fmt.Errorf("%w: %q", ErrInvalidSource, spec)
	}

	// Before any colon splitting: "https://..." would otherwise read as a two-field key
	// with the repository "https".
	if urlScheme.MatchString(spec) {
		if err := validateHubURL(spec); err != nil {
			return Source{}, err
		}
		if mmprojURL != "" {
			if err := validateHubURL(mmprojURL); err != nil {
				return Source{}, fmt.Errorf("mmproj url: %w", err)
			}
		}
		return Source{spec: spec, isURL: true, mmprojURL: mmprojURL}, nil
	}

	if mmprojURL != "" {
		return Source{}, fmt.Errorf("%w: a compact key names its mmproj quantization as a fourth field, so it takes no mmproj url", ErrInvalidSource)
	}

	// The field count alone disambiguates: a lone field can only be a repository, and a
	// pair can only be repository plus quantization, since the framework never appears
	// without one.
	var src Source
	switch parts := strings.Split(spec, ":"); len(parts) {
	case 1:
		src = Source{repoID: parts[0], quantization: defaultQuantization, quantizationDefaulted: true}
	case 2:
		src = Source{repoID: parts[0], quantization: parts[1]}
	case 3:
		src = Source{repoID: parts[1], quantization: parts[2]}
	case 4:
		src = Source{repoID: parts[1], quantization: parts[2], mmprojQuantization: parts[3]}
	default:
		return Source{}, fmt.Errorf("%w: %q has too many fields, expected [<framework>:]<repo>[:<quantization>[:<mmproj quantization>]]", ErrInvalidSource, spec)
	}
	src.spec = spec

	if err := validateRepoID(src.repoID); err != nil {
		return Source{}, err
	}
	if err := validateQuantization(src.quantization); err != nil {
		return Source{}, err
	}
	if src.mmprojQuantization != "" {
		if err := validateQuantization(src.mmprojQuantization); err != nil {
			return Source{}, fmt.Errorf("mmproj %w", err)
		}
	}
	return src, nil
}

// Variables are the handler's download variables for this source. The handler's
// --model-url takes either form, so a compact key travels in model_url too.
func (s Source) Variables() map[string]string {
	variables := map[string]string{"model_url": s.spec}
	if s.mmprojURL != "" {
		variables["model_mmproj_url"] = s.mmprojURL
	}
	return variables
}

// RepoID is the "<owner>/<repo>" the model comes from. Empty for the URL form, where the
// handler derives it from the path.
func (s Source) RepoID() string { return s.repoID }

// Quantization is the quantization asked for, or the default one when a key named only a
// repository. QuantizationDefaulted tells the two apart, because a silently substituted
// quantization is worth reporting to whoever asked.
func (s Source) Quantization() string        { return s.quantization }
func (s Source) QuantizationDefaulted() bool { return s.quantizationDefaulted }
func (s Source) MmprojQuantization() string  { return s.mmprojQuantization }
func (s Source) IsURL() bool                 { return s.isURL }
func (s Source) String() string              { return s.spec }

func validateHubURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %q is not a url", ErrInvalidSource, raw)
	}
	switch {
	case parsed.Scheme != "https" && parsed.Scheme != "http":
		return fmt.Errorf("%w: %q is not served over http", ErrInvalidSource, raw)
	// A lookalike domain, credentials hiding the real host, or an explicit port: the host
	// is compared rather than matched, so none of the three passes.
	case !strings.EqualFold(parsed.Hostname(), hubHost) || parsed.User != nil || parsed.Port() != "":
		return fmt.Errorf("%w: %q is not served by %s", ErrInvalidSource, raw, hubHost)
	case !strings.HasSuffix(parsed.Path, ".gguf"):
		return fmt.Errorf("%w: %q does not name a .gguf file", ErrInvalidSource, raw)
	}
	return nil
}

// validateRepoID keeps the id a name the Hub could have issued, which is also what keeps
// it safe as a path: the handler derives <models dir>/<repo id> from it, and its delete
// action removes that tree, so a value such as "../../etc" must never get that far.
func validateRepoID(repoID string) error {
	parts := strings.Split(repoID, "/")
	if len(parts) > 2 {
		return fmt.Errorf("%w: repository %q has too many parts, expected <owner>/<repo> or <repo>", ErrInvalidSource, repoID)
	}
	for _, part := range parts {
		if !hubName.MatchString(part) {
			return fmt.Errorf("%w: repository %q is not a Hugging Face name", ErrInvalidSource, repoID)
		}
	}
	return nil
}

// validateQuantization rejects a value that names a file rather than a quantization. A
// quantization is a single token, "Q4_0" or "BF16", never a path: an installed model's id
// fed back to this endpoint lands here, and saying so beats asking the Hub for a
// repository that cannot exist.
func validateQuantization(quantization string) error {
	if quantization == "" {
		return fmt.Errorf("%w: quantization is empty", ErrInvalidSource)
	}
	if strings.ContainsAny(quantization, "/") {
		return fmt.Errorf("%w: quantization %q reads as a path, so this looks like a model id rather than a source", ErrInvalidSource, quantization)
	}
	return nil
}
