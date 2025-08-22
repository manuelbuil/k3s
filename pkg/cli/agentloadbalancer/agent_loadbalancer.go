package agentloadbalancer

import (

	"fmt"

	"github.com/urfave/cli/v2"

)

func Status(app *cli.Context) error {
	fmt.Println("Checking agent load balancer status...")
	return nil
}