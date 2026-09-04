package main

import (
	"bufio"
	"fmt"
	"os"

	//"github.com/72sevenzy2/web-crawler"
)

func main() {
	fmt.Println("test")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("aa")
		safe := scanner.Scan()
		if !safe {
			fmt.Println("not safe")
		}
	}
}
