// Package kustomize renders kustomization directories using the krusty engine.
package kustomize

import (
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Render builds the kustomization at path with helm support enabled
// and returns the rendered multi-document YAML.
func Render(path string) ([]byte, error) {
	opts := krusty.MakeDefaultOptions()
	opts.PluginConfig = types.MakePluginConfig(
		types.PluginRestrictionsNone,
		types.BploUseStaticallyLinked,
	)
	opts.PluginConfig.HelmConfig = types.HelmConfig{
		Enabled: true,
		Command: "helm",
	}

	k := krusty.MakeKustomizer(opts)
	m, err := k.Run(filesys.MakeFsOnDisk(), path)
	if err != nil {
		return nil, err
	}

	return m.AsYaml()
}
