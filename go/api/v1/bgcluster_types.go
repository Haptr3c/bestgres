//go:generate controller-gen object paths="."

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=bgclusters,scope=Namespaced,shortName=bgclu
// +groupName=bestgres.io

// BGCluster is the Schema for the bgclusters API
type BGCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BGClusterSpec   `json:"spec,omitempty"`
	Status BGClusterStatus `json:"status,omitempty"`
}

// BGClusterSpec defines the desired state of BGCluster
type BGClusterSpec struct {
	// The number of instances in the cluster
	// Multiple instances will configure themselves as a Patroni HA cluster
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Instances int32 `json:"instances"`
	// +kubebuilder:validation:Required
	VolumeSpec VolumeSpec `json:"volumeSpec"`
	// +kubebuilder:validation:Required
	Image ImageSpec `json:"image"`
	// +kubebuilder:default="INFO"
	PatroniLogLevel string `json:"patroniLogLevel,omitempty"`
	// +kubebuilder:default={}
	BootstrapSQL []string `json:"bootstrapSQL,omitempty"`
}

// ImageSpec defines the Image-specific configuration
type ImageSpec struct {
	// +kubebuilder:validation:Required
	Tag string `json:"tag"`
	// +kubebuilder:default="/home/postgres"
	WorkingDir string `json:"workingDir,omitempty"`
	// The command to run when the container starts
	// Make sure to set this if the image does not use the default spilo command
	// +kubebuilder:default={"/bin/sh", "/launch.sh", "init"}
	Command []string `json:"command,omitempty"`
}

// VolumeSpec defines the volume configuration
type VolumeSpec struct {
	// The size of the persistent volume
	// +kubebuilder:validation:Required
	PersistentVolumeSize string `json:"persistentVolumeSize"`
	// The storage class to use for the persistent volume
	// +kubebuilder:validation:Required
	StorageClass string `json:"storageClass"`
}

// BGClusterStatus defines the observed state of BGCluster
type BGClusterStatus struct {
	Nodes []string `json:"nodes"`
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BGClusterSpec) DeepCopyInto(out *BGClusterSpec) {
	*out = *in
	out.Image = in.Image.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BGClusterSpec.
func (in *BGClusterSpec) DeepCopy() *BGClusterSpec {
	if in == nil {
		return nil
	}
	out := new(BGClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopy is a deepcopy function for ImageSpec
func (in *ImageSpec) DeepCopy() ImageSpec {
	out := *in
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		copy(out.Command, in.Command)
	}
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BGClusterStatus) DeepCopyInto(out *BGClusterStatus) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BGClusterStatus.
func (in *BGClusterStatus) DeepCopy() *BGClusterStatus {
	if in == nil {
		return nil
	}
	out := new(BGClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// BGClusterList contains a list of BGCluster
type BGClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGCluster{}, &BGClusterList{})
}