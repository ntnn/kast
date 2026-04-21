// Package order sorts Kubernetes resource manifests by kind priority.
package order

import (
	"bytes"
	"sort"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// kindPriority defines the apply order for Kubernetes resource kinds.
// Lower values are applied first. Kinds not in the map get a default
// priority in the middle of the range.
var kindPriority = map[string]int{
	"Namespace":                10,
	"ResourceQuota":            20,
	"LimitRange":               25,
	"CustomResourceDefinition": 30,
	"ServiceAccount":           40,
	"ClusterRole":              50,
	"ClusterRoleBinding":       60,
	"Role":                     70,
	"RoleBinding":              80,
	"ConfigMap":                90,
	"Secret":                   100,
	"PersistentVolumeClaim":    110,
	"PersistentVolume":         115,
	"Service":                  120,
	"Deployment":               130,
	"StatefulSet":              140,
	"DaemonSet":                150,
	"Job":                      160,
	"CronJob":                  170,
	"IngressClass":             175,
	"Ingress":                  180,
}

const defaultPriority = 125

type manifest struct {
	node     *yaml.RNode
	kind     string
	priority int
}

// Sort orders multi-document YAML by kind priority for apply operations.
func Sort(data []byte) ([]byte, error) {
	return sortManifests(data, false)
}

// SortReverse orders multi-document YAML in reverse kind priority for
// delete operations.
func SortReverse(data []byte) ([]byte, error) {
	return sortManifests(data, true)
}

func sortManifests(data []byte, reverse bool) ([]byte, error) {
	reader := &kio.ByteReader{
		Reader:                bytes.NewReader(data),
		OmitReaderAnnotations: true,
	}

	nodes, err := reader.Read()
	if err != nil {
		return nil, err
	}

	manifests := make([]manifest, 0, len(nodes))
	for _, node := range nodes {
		kind := node.GetKind()
		p, ok := kindPriority[kind]
		if !ok {
			p = defaultPriority
		}
		manifests = append(manifests, manifest{
			node:     node,
			kind:     kind,
			priority: p,
		})
	}

	sort.SliceStable(manifests, func(i, j int) bool {
		if reverse {
			return manifests[i].priority > manifests[j].priority
		}
		return manifests[i].priority < manifests[j].priority
	})

	var buf bytes.Buffer
	for i, m := range manifests {
		if i > 0 {
			buf.WriteString("---\n")
		}
		s, err := m.node.String()
		if err != nil {
			return nil, err
		}
		buf.WriteString(s)
	}
	return buf.Bytes(), nil
}
