package helpers

type EnvVars map[string]string

func (e EnvVars) AsList() []string {
	list := make([]string, 0, len(e))
	for k, v := range e {
		list = append(list, k+"="+v)
	}
	return list
}
