// This file is part of arduino-app-cli.
//
// Copyright 2020 ARDUINO SA (http://www.arduino.cc/)
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package i18n

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/leonelquinteros/gotext"
	"github.com/stretchr/testify/require"
)

func setPo(poFile string) {
	dict := gotext.NewPo()
	dict.Parse([]byte(poFile))
	SetLocale(dict)
}

func TestPoTranslation(t *testing.T) {
	setPo(`
		msgid "test-key-ok"
		msgstr "test-key-translated"
	`)
	require.Equal(t, "test-key", Tr("test-key"))
	require.Equal(t, "test-key-translated", Tr("test-key-ok"))
}

func TestNoLocaleSet(t *testing.T) {
	locale = gotext.NewPo()
	require.Equal(t, "test-key", Tr("test-key"))
}

func TestTranslationWithVariables(t *testing.T) {
	setPo(`
		msgid "test-key-ok %s"
		msgstr "test-key-translated %s"
	`)
	require.Equal(t, "test-key", Tr("test-key"))
	require.Equal(t, "test-key-translated message", Tr("test-key-ok %s", "message"))
}

func TestTranslationInTemplate(t *testing.T) {
	setPo(`
		msgid "test-key"
		msgstr "test-key-translated %s"
	`)

	tpl, err := template.New("test-template").Funcs(template.FuncMap{
		"tr": Tr,
	}).Parse(`{{ tr "test-key" .Value }}`)
	require.NoError(t, err)

	data := struct {
		Value string
	}{
		"value",
	}
	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, data))

	require.Equal(t, "test-key-translated value", buf.String())
}

func TestTranslationWithQuotedStrings(t *testing.T) {
	setPo(`
		msgid "test-key \"quoted\""
		msgstr "test-key-translated"
	`)

	require.Equal(t, "test-key-translated", Tr("test-key \"quoted\""))
	require.Equal(t, "test-key-translated", Tr(`test-key "quoted"`))
}

func TestTranslationWithLineBreaks(t *testing.T) {
	setPo(`
		msgid "test-key \"quoted\"\n"
		"new line"
		msgstr "test-key-translated"
	`)

	require.Equal(t, "test-key-translated", Tr("test-key \"quoted\"\nnew line"))
	require.Equal(t, "test-key-translated", Tr(`test-key "quoted"
new line`))
}
