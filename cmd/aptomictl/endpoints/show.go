package endpoints

import (
	"fmt"
	"github.com/Aptomi/aptomi/pkg/client/rest"
	"github.com/Aptomi/aptomi/pkg/client/rest/http"
	"github.com/Aptomi/aptomi/pkg/config"
	"github.com/spf13/cobra"
)

func newShowCommand(cfg *config.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "endpoints show",
		Long:  "endpoints show long",

		Run: func(cmd *cobra.Command, args []string) {
			endpoints, err := rest.New(cfg, http.NewClient(cfg)).Endpoints().Show()
			if err != nil {
				panic(fmt.Sprintf("Error while requesting endpoints: %s", err))
			}

			// todo(slukjanov): replace with -o yaml / json / etc handler
			fmt.Println(endpoints)
		},
	}
}
