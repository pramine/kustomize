// Code generated by pluginator on InventoryTransformer; DO NOT EDIT.
package builtin

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/pkg/resource"

	"sigs.k8s.io/kustomize/pkg/hasher"
	"sigs.k8s.io/kustomize/pkg/ifc"
	"sigs.k8s.io/kustomize/pkg/inventory"
	"sigs.k8s.io/kustomize/pkg/resid"
	"sigs.k8s.io/kustomize/pkg/resmap"
	"sigs.k8s.io/kustomize/pkg/types"
	"sigs.k8s.io/yaml"
)

type InventoryTransformerPlugin struct {
	ldr       ifc.Loader
	rf        *resmap.Factory
	Policy    string `json:"policy,omitempty" yaml:"policy,omitempty"`
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

func NewInventoryTransformerPlugin() *InventoryTransformerPlugin {
	return &InventoryTransformerPlugin{}
}

func (p *InventoryTransformerPlugin) Config(
	ldr ifc.Loader, rf *resmap.Factory, c []byte) (err error) {
	p.ldr = ldr
	p.rf = rf
	err = yaml.Unmarshal(c, p)
	if err != nil {
		return err
	}
	if p.Policy == "" {
		p.Policy = types.GarbageIgnore.String()
	}
	if p.Policy != types.GarbageCollect.String() &&
		p.Policy != types.GarbageIgnore.String() {
		return fmt.Errorf(
			"unrecognized garbagePolicy '%s'", p.Policy)
	}
	return nil
}

// Transform generates an inventory object from the input ResMap.
// This ConfigMap supports the pruning command in
// the client side tool proposed here:
// https://github.com/kubernetes/enhancements/pull/810
//
// The inventory data is written to the ConfigMap's
// annotations, rather than to the key-value pairs in
// the ConfigMap's data field, since
//   1. Keys in a ConfigMap's data field are too
//      constrained for this purpose.
//   2. Using annotations allow any object to be used,
//      not just a ConfigMap, should some other object
//      (e.g. some App object) become more desirable
//      for this purpose.
func (p *InventoryTransformerPlugin) Transform(m resmap.ResMap) error {

	inv, h, err := makeInventory(m)
	if err != nil {
		return err
	}

	args := types.ConfigMapArgs{}
	args.Name = p.Name
	args.Namespace = p.Namespace
	opts := &types.GeneratorOptions{
		Annotations: make(map[string]string),
	}
	opts.Annotations[inventory.HashAnnotation] = h
	err = inv.UpdateAnnotations(opts.Annotations)
	if err != nil {
		return err
	}

	cm, err := p.rf.RF().MakeConfigMap(p.ldr, opts, &args)
	if err != nil {
		return err
	}

	if p.Policy == types.GarbageCollect.String() {
		for byeBye := range m {
			delete(m, byeBye)
		}
	}

	id := cm.Id()
	if _, ok := m[id]; ok {
		return fmt.Errorf(
			"id '%v' already used; use a different name", id)
	}
	m[id] = cm
	return nil
}

func makeInventory(m resmap.ResMap) (
	inv *inventory.Inventory, hash string, err error) {
	inv = inventory.NewInventory()
	var keys []string
	for _, r := range m {
		ns := getNamespace(r)
		item := resid.NewItemId(r.GetGvk(), ns, r.GetName())
		if _, ok := inv.Current[item]; ok {
			return nil, "", fmt.Errorf(
				"item '%v' already in inventory", item)
		}
		inv.Current[item] = computeRefs(r, m)
		keys = append(keys, item.String())
	}
	h, err := hasher.SortArrayAndComputeHash(keys)
	return inv, h, err
}

func getNamespace(r *resource.Resource) string {
	ns, err := r.GetFieldValue("metadata.namespace")
	if err != nil && !strings.Contains(err.Error(), "no field named") {
		panic(err)
	}
	return ns
}

func computeRefs(r *resource.Resource, m resmap.ResMap) (refs []resid.ItemId) {
	for _, refid := range r.GetRefBy() {
		ref := m[refid]
		ns := getNamespace(ref)
		refs = append(refs, resid.NewItemId(ref.GetGvk(), ns, ref.GetName()))
	}
	return
}
