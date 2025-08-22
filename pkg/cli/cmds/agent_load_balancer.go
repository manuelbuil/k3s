package cmds

import (
	"github.com/urfave/cli/v2"
)

const AgentLoadBalancerCommand = "agent-loadbalancer"

// NewAgentLoadBalancerCommands creates the command for the CLI
func NewAgentLoadBalancerCommands(status func(ctx *cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:  AgentLoadBalancerCommand,
		Usage: "Control and get status of the agent load balancer",
		Subcommands: []*cli.Command{
			{
				Name:   "status",
				Usage:  "Print current status of the load balancer",
				Action: status,
				Flags: []cli.Flag{
					DataDirFlag,
				},
			},
		},
	}
}