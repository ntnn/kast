package resources

import (
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Kustomize builds the kustomization at the given path with helm
// support enabled and returns the rendered manifests.
func Kustomize(path string) ([]*resource.Resource, error) {
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

	return m.Resources(), nil
}
