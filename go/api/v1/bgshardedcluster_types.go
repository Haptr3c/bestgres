//go:generate controller-gen object paths="."

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=bgshardedclusters,scope=Namespaced,shortName=bgsclu
// +groupName=bestgres.io

// BGShardedCluster is the Schema for the bgshardedclusters API
type BGShardedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BGShardedClusterSpec   `json:"spec,omitempty"`
	Status BGShardedClusterStatus `json:"status,omitempty"`
}

// BGShardedClusterSpec defines the desired state of BGShardedCluster
type BGShardedClusterSpec struct {
	// +kubebuilder:validation:Required
	// Number of shards in the cluster
	Shards int32 `json:"shards"`
	// +kubebuilder:validation:Required
	// Coordinator node configuration
	Coordinator BGClusterSpec `json:"coordinator"`
	// +kubebuilder:validation:Required
	// Worker nodes configuration
	Workers BGClusterSpec `json:"workers"`
}

// BGShardedClusterStatus defines the observed state of BGShardedCluster
type BGShardedClusterStatus struct {
	// Status of the sharded cluster
	Status string `json:"status"`
	// Names of the coordinator and worker BGClusters
	CoordinatorCluster string   `json:"coordinatorCluster"`
	WorkerClusters     []string `json:"workerClusters"`
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BGShardedClusterSpec) DeepCopyInto(out *BGShardedClusterSpec) {
	*out = *in
	in.Coordinator.DeepCopyInto(&out.Coordinator)
	in.Workers.DeepCopyInto(&out.Workers)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BGShardedClusterSpec.
func (in *BGShardedClusterSpec) DeepCopy() *BGShardedClusterSpec {
	if in == nil {
		return nil
	}
	out := new(BGShardedClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BGShardedClusterStatus) DeepCopyInto(out *BGShardedClusterStatus) {
	*out = *in
	if in.WorkerClusters != nil {
		in, out := &in.WorkerClusters, &out.WorkerClusters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BGShardedClusterStatus.
func (in *BGShardedClusterStatus) DeepCopy() *BGShardedClusterStatus {
	if in == nil {
		return nil
	}
	out := new(BGShardedClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// BGShardedClusterList contains a list of BGShardedCluster
type BGShardedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGShardedCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGShardedCluster{}, &BGShardedClusterList{})
}