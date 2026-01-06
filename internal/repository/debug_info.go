package repository

import "strings"

type RepositoryDebugInfo struct {
	Type                        string
	BaseDir                     string
	ResourceFormat              string
	RemoteURL                   string
	RemoteBranch                string
	RemoteProvider              string
	RemoteAuth                  string
	RemoteAutoSync              *bool
	RemoteTLSInsecureSkipVerify *bool
}

func (m *GitResourceRepositoryManager) DebugInfo() RepositoryDebugInfo {
	info := RepositoryDebugInfo{Type: "git"}
	if m == nil {
		return info
	}
	if m.fs != nil {
		info.BaseDir = strings.TrimSpace(m.fs.BaseDir)
		info.ResourceFormat = string(normalizeResourceFormat(m.fs.ResourceFormat))
	}

	cfg := m.config
	if cfg == nil {
		return info
	}
	if cfg.Local != nil && strings.TrimSpace(cfg.Local.BaseDir) != "" {
		info.BaseDir = strings.TrimSpace(cfg.Local.BaseDir)
	}
	if cfg.Remote != nil {
		info.RemoteURL = RedactGitCredentials(strings.TrimSpace(cfg.Remote.URL))
		info.RemoteBranch = strings.TrimSpace(cfg.Remote.Branch)
		info.RemoteProvider = strings.TrimSpace(cfg.Remote.Provider)
		info.RemoteAuth = gitAuthMethodLabel(cfg.Remote.Auth)
		if cfg.Remote.AutoSync != nil {
			value := *cfg.Remote.AutoSync
			info.RemoteAutoSync = &value
		}
		if cfg.Remote.TLS != nil {
			value := cfg.Remote.TLS.InsecureSkipVerify
			info.RemoteTLSInsecureSkipVerify = &value
		}
	}

	return info
}

func (m *FileSystemResourceRepositoryManager) DebugInfo() RepositoryDebugInfo {
	info := RepositoryDebugInfo{Type: "filesystem"}
	if m == nil {
		return info
	}
	info.BaseDir = strings.TrimSpace(m.BaseDir)
	info.ResourceFormat = string(normalizeResourceFormat(m.ResourceFormat))
	return info
}

func RedactGitCredentials(message string) string {
	return redactGitCredentials(message)
}

func gitAuthMethodLabel(cfg *GitResourceRepositoryRemoteAuthConfig) string {
	if cfg == nil {
		return "none"
	}
	if cfg.BasicAuth != nil {
		return "basic_auth"
	}
	if cfg.AccessKey != nil {
		return "access_key"
	}
	if cfg.SSH != nil {
		return "ssh"
	}
	return "none"
}
