package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/72sevenzy2/web-crawler"
)

func main() {
	d := flag.Int("depth", 10, "-depth <int>")
	r := flag.Int("retries", 5, "-retries <int>")
	t := flag.Bool("cross-domains", true, "-cross-domains <bool>")
	c := crawler.NewCrawler(*d, *t, *r)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		safe := scanner.Scan()
		if !safe {
			if err := scanner.Err(); err != nil {
				fmt.Println("err:", err.Error())
			}
			break
		}
		parts := strings.Fields(scanner.Text())

		switch strings.ToLower(parts[0]) {
		case "crawl":
			if len(parts) < 2 {
				fmt.Println("include a url to crawl through.")
				continue
			}

			c.Start(context.Background(), parts[1])
		default:
			fmt.Println("invalid command")
			continue
		}
	}
}
