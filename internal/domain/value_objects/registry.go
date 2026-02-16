package value_objects

const maxNormalizedLen = 40

type RegistryConfig struct {
	host      string
	namespace string
	arch      string
	os        string
}

func NewRegistryConfig(host, namespace, arch, osType string) *RegistryConfig {
	return &RegistryConfig{
		host:      host,
		namespace: namespace,
		arch:      arch,
		os:        osType,
	}
}

func (r *RegistryConfig) Host() string {
	return r.host
}

func (r *RegistryConfig) Namespace() string {
	return r.namespace
}

func (r *RegistryConfig) Arch() string {
	return r.arch
}

func (r *RegistryConfig) Os() string {
	return r.os
}
