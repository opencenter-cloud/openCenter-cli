package gitops

import v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"

func managedServices(cfg v2.Config) v2.ServiceMap {
	if len(cfg.OpenCenter.ManagedServices) > 0 {
		return cfg.OpenCenter.ManagedServices
	}
	return cfg.OpenCenter.LegacyManaged
}
