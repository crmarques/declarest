package common

func ResolvePathInput(pathFlag string, args []string, required bool) (string, error) {
	var pathArg string
	if len(args) > 0 {
		pathArg = args[0]
	}

	if pathFlag != "" && pathArg != "" && pathFlag != pathArg {
		return "", ValidationError("path mismatch between positional argument and --path", nil)
	}

	path := pathArg
	if pathFlag != "" {
		path = pathFlag
	}

	if required && path == "" {
		return "", ValidationError("path is required", nil)
	}

	return path, nil
}
