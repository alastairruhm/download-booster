package main

import (
	"fmt"
	"os"

	"github.com/alastairruhm/download-booster/proxy"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "download-boost"
	app.Usage = "boost download in internal network"

	var get proxy.Get

	app.Before = func(c *cli.Context) error {
		// check url validation
		// fmt.Println(c.Args().First())
		// check range support
		supportRange, err := get.CheckAcceptRangeSupport(c.Args().First())
		if err != nil {
			return err
		}
		if supportRange == false {
			fmt.Printf("Server %s doesn't support Range.\n", get.Header.Get("Server"))
			os.Exit(-1)
		}
		fmt.Printf("Server %s support Range by %s.\n", get.Header.Get("Server"), get.Header.Get("Accept-Ranges"))
		return nil
	}

	app.Action = func(c *cli.Context) error {
		err := get.DownloadInParallel(c.Args().First())
		if err != nil {
			return err
		}
		return nil
	}

	app.Run(os.Args)
}
