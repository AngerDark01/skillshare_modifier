package main

import (
	"skillshare/internal/config"
)

func reconcileProjectRemoteSkills(runtime *projectRuntime) error {
	return config.ReconcileProjectSkills(runtime.root, runtime.config, runtime.skillsStore, runtime.sourcePath)
}
