package cluster

import (
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func Test_UnitCompareCriticalControlArgs(t *testing.T) {
	tests := []struct {
		name    string
		local   config.CriticalControlArgs
		cluster config.CriticalControlArgs
		wantErr bool
	}{
		{
			name:    "identical empty config matches",
			local:   config.CriticalControlArgs{},
			cluster: config.CriticalControlArgs{},
			wantErr: false,
		},
		{
			name:    "identical extra config matches",
			local:   config.CriticalControlArgs{CriticalExtraConfig: `{"cni":["canal"]}`},
			cluster: config.CriticalControlArgs{CriticalExtraConfig: `{"cni":["canal"]}`},
			wantErr: false,
		},
		{
			name:    "divergent extra config fails",
			local:   config.CriticalControlArgs{CriticalExtraConfig: `{"ingressController":["traefik"]}`},
			cluster: config.CriticalControlArgs{CriticalExtraConfig: `{"ingressController":["ingress-nginx"]}`},
			wantErr: true,
		},
		{
			name: "down-level cluster (empty extra config) is tolerated",
			// A joining new server has the field populated, but the existing
			// cluster is older and served an empty value: must not block the join.
			local:   config.CriticalControlArgs{CriticalExtraConfig: `{"cni":["canal"]}`},
			cluster: config.CriticalControlArgs{CriticalExtraConfig: ""},
			wantErr: false,
		},
		{
			name:    "divergent system-default-registry fails",
			local:   config.CriticalControlArgs{SystemDefaultRegistry: "registry.example.com"},
			cluster: config.CriticalControlArgs{SystemDefaultRegistry: "other.example.com"},
			wantErr: true,
		},
		{
			name:    "divergent disable-kube-proxy fails",
			local:   config.CriticalControlArgs{DisableKubeProxy: true},
			cluster: config.CriticalControlArgs{DisableKubeProxy: false},
			wantErr: true,
		},
		{
			name:    "divergent servicelb-namespace fails",
			local:   config.CriticalControlArgs{ServiceLBNamespace: "kube-system"},
			cluster: config.CriticalControlArgs{ServiceLBNamespace: "custom"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compareCriticalControlArgs(tt.local, tt.cluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("compareCriticalControlArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
