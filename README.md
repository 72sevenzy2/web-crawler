<h1 align="center">concurrently-safe web crawler for link traversal.</h1>
<br>
<h2>usage:</h2>
  
  ```
  package main

  import (
      "context"
      "time"
    	"github.com/72sevenzy2/web-crawler"
  )

  func main() {
    wc := crawler.NewCrawler(10, true)
    /*
      NewCrawler() takes in a max depth, which is the number of times to traverse through links, and a boolean to which allow traversing through external domains other than the host.
    */

    ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
    defer cancel()

    wc.Start(ctx, "https://jsonplaceholder.typicode.com/guide/", 0)
    /*
      wc.Start() takes in a url, and a start depth in which the crawler is to start from, so each time it scours for links in given url,
      it gradually increments start depth untill it has exceeded the max depth.
    */
  }
  ```
