package templates

import (
	"bytes"
	"net/url"
	"text/template"

	"github.com/rancher/wharfie/pkg/registries"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
)

type ContainerdRuntimeConfig struct {
	RuntimeType string
	BinaryName  string
}

type ContainerdConfig struct {
	NodeConfig            *config.Node
	DisableCgroup         bool
	SystemdCgroup         bool
	IsRunningInUserNS     bool
	EnableUnprivileged    bool
	NoDefaultEndpoint     bool
	NonrootDevices        bool
	PrivateRegistryConfig *registries.Registry
	ExtraRuntimes         map[string]ContainerdRuntimeConfig
	Program               string
}

type RegistryEndpoint struct {
	OverridePath bool
	URL          *url.URL
	Rewrites     map[string]string
	Config       registries.RegistryConfig
}

type HostConfig struct {
	Default   *RegistryEndpoint
	Program   string
	Endpoints []RegistryEndpoint
}

const ContainerdConfigTemplate = `
{{- /* */ -}}
# File generated by {{ .Program }}. DO NOT EDIT. Use config.toml.tmpl instead.
version = 2
root = {{ printf "%q" .NodeConfig.Containerd.Root }}
state = {{ printf "%q" .NodeConfig.Containerd.State }}

[plugins."io.containerd.internal.v1.opt"]
  path = {{ printf "%q" .NodeConfig.Containerd.Opt }}

[plugins."io.containerd.grpc.v1.cri"]
  stream_server_address = "127.0.0.1"
  stream_server_port = "10010"
  enable_selinux = {{ .NodeConfig.SELinux }}
  enable_unprivileged_ports = {{ .EnableUnprivileged }}
  enable_unprivileged_icmp = {{ .EnableUnprivileged }}
  device_ownership_from_security_context = {{ .NonrootDevices }}

{{- if .DisableCgroup}}
  disable_cgroup = true
{{end}}
{{- if .IsRunningInUserNS }}
  disable_apparmor = true
  restrict_oom_score_adj = true
{{end}}

{{- if .NodeConfig.AgentConfig.PauseImage }}
  sandbox_image = "{{ .NodeConfig.AgentConfig.PauseImage }}"
{{end}}

{{- if .NodeConfig.AgentConfig.Snapshotter }}
[plugins."io.containerd.grpc.v1.cri".containerd]
  snapshotter = "{{ .NodeConfig.AgentConfig.Snapshotter }}"
  disable_snapshot_annotations = {{ if eq .NodeConfig.AgentConfig.Snapshotter "stargz" }}false{{else}}true{{end}}
  {{ if .NodeConfig.DefaultRuntime }}default_runtime_name = "{{ .NodeConfig.DefaultRuntime }}"{{end}}
{{ if eq .NodeConfig.AgentConfig.Snapshotter "stargz" }}
{{ if .NodeConfig.AgentConfig.ImageServiceSocket }}
[plugins."io.containerd.snapshotter.v1.stargz"]
cri_keychain_image_service_path = "{{ .NodeConfig.AgentConfig.ImageServiceSocket }}"
[plugins."io.containerd.snapshotter.v1.stargz".cri_keychain]
enable_keychain = true
{{end}}

[plugins."io.containerd.snapshotter.v1.stargz".registry]
  config_path = {{ printf "%q" .NodeConfig.Containerd.Registry }}

{{ if .PrivateRegistryConfig }}
{{range $k, $v := .PrivateRegistryConfig.Configs }}
{{ if $v.Auth }}
[plugins."io.containerd.snapshotter.v1.stargz".registry.configs."{{$k}}".auth]
  {{ if $v.Auth.Username }}username = {{ printf "%q" $v.Auth.Username }}{{end}}
  {{ if $v.Auth.Password }}password = {{ printf "%q" $v.Auth.Password }}{{end}}
  {{ if $v.Auth.Auth }}auth = {{ printf "%q" $v.Auth.Auth }}{{end}}
  {{ if $v.Auth.IdentityToken }}identitytoken = {{ printf "%q" $v.Auth.IdentityToken }}{{end}}
{{end}}
{{end}}
{{end}}
{{end}}
{{end}}

{{- if not .NodeConfig.NoFlannel }}
[plugins."io.containerd.grpc.v1.cri".cni]
  bin_dir = {{ printf "%q" .NodeConfig.AgentConfig.CNIBinDir }}
  conf_dir = {{ printf "%q" .NodeConfig.AgentConfig.CNIConfDir }}
{{end}}

{{- if or .NodeConfig.Containerd.BlockIOConfig .NodeConfig.Containerd.RDTConfig }}
[plugins."io.containerd.service.v1.tasks-service"]
  {{ if .NodeConfig.Containerd.BlockIOConfig }}blockio_config_file = {{ printf "%q" .NodeConfig.Containerd.BlockIOConfig }}{{end}}
  {{ if .NodeConfig.Containerd.RDTConfig }}rdt_config_file = {{ printf "%q" .NodeConfig.Containerd.RDTConfig }}{{end}}
{{end}}

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
  runtime_type = "io.containerd.runc.v2"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
  SystemdCgroup = {{ .SystemdCgroup }}

[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = {{ printf "%q" .NodeConfig.Containerd.Registry }}

{{ if .PrivateRegistryConfig }}
{{range $k, $v := .PrivateRegistryConfig.Configs }}
{{ if $v.Auth }}
[plugins."io.containerd.grpc.v1.cri".registry.configs."{{$k}}".auth]
  {{ if $v.Auth.Username }}username = {{ printf "%q" $v.Auth.Username }}{{end}}
  {{ if $v.Auth.Password }}password = {{ printf "%q" $v.Auth.Password }}{{end}}
  {{ if $v.Auth.Auth }}auth = {{ printf "%q" $v.Auth.Auth }}{{end}}
  {{ if $v.Auth.IdentityToken }}identitytoken = {{ printf "%q" $v.Auth.IdentityToken }}{{end}}
{{end}}
{{end}}
{{end}}

{{range $k, $v := .ExtraRuntimes}}
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes."{{$k}}"]
  runtime_type = "{{$v.RuntimeType}}"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes."{{$k}}".options]
  BinaryName = "{{$v.BinaryName}}"
  SystemdCgroup = {{ $.SystemdCgroup }}
{{end}}
`

var HostsTomlHeader = "# File generated by " + version.Program + ". DO NOT EDIT.\n"

const HostsTomlTemplate = `
{{- /* */ -}}
# File generated by {{ .Program }}. DO NOT EDIT.
{{ with $e := .Default }}
{{- if $e.URL }}
server = "{{ $e.URL }}"
capabilities = ["pull", "resolve", "push"]
{{ end }}
{{- if $e.Config.TLS }}
{{- if $e.Config.TLS.CAFile }}
ca = [{{ printf "%q" $e.Config.TLS.CAFile }}]
{{- end }}
{{- if or $e.Config.TLS.CertFile $e.Config.TLS.KeyFile }}
client = [[{{ printf "%q" $e.Config.TLS.CertFile }}, {{ printf "%q" $e.Config.TLS.KeyFile }}]]
{{- end }}
{{- if $e.Config.TLS.InsecureSkipVerify }}
skip_verify = true
{{- end }}
{{ end }}
{{ end }}
[host]
{{ range $e := .Endpoints -}}
[host."{{ $e.URL }}"]
  capabilities = ["pull", "resolve"]
  {{- if $e.OverridePath }}
  override_path = true
  {{- end }}
{{- if $e.Config.TLS }}
  {{- if $e.Config.TLS.CAFile }}
  ca = [{{ printf "%q" $e.Config.TLS.CAFile }}]
  {{- end }}
  {{- if or $e.Config.TLS.CertFile $e.Config.TLS.KeyFile }}
  client = [[{{ printf "%q" $e.Config.TLS.CertFile }}, {{ printf "%q" $e.Config.TLS.KeyFile }}]]
  {{- end }}
  {{- if $e.Config.TLS.InsecureSkipVerify }}
  skip_verify = true
  {{- end }}
{{ end }}
{{- if $e.Rewrites }}
  [host."{{ $e.URL }}".rewrite]
  {{- range $pattern, $replace := $e.Rewrites }}
    "{{ $pattern }}" = "{{ $replace }}"
  {{- end }}
{{ end }}
{{ end -}}
`

func ParseTemplateFromConfig(templateBuffer string, config interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Funcs(templateFuncs).Parse(templateBuffer))
	template.Must(t.New("base").Parse(ContainerdConfigTemplate))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return out.String(), nil
}

func ParseHostsTemplateFromConfig(templateBuffer string, config interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Funcs(templateFuncs).Parse(templateBuffer))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return out.String(), nil
}
