package profilebootstrap

func ResolveRepositoryPaths() ([]string, error) {
	resolved, err := ResolveUnifiedConfig(nil)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Effective == nil {
		return nil, nil
	}
	return append([]string(nil), resolved.Effective.App.Repositories...), nil
}
