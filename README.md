<h1 align="center">concurrently-safe web crawler for traversing through links.</h1>
<br>
<h2>configurable options to the crawler:</h2>
<h3>- custom max retry limits.</h3>
<h3>- toggleable option for whether crawler reaches non-host url domains.</h3>
<h3>- depth size, for which how deep into host url the crawler can crawl through.</h3>
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
    wc := crawler.NewCrawler(10, true, 5)
    /*
      NewCrawler() takes in a max depth, which is the number of times to traverse through links, and a boolean to which allow traversing through external domains other than the host, and also takes in a number of retries if one crawl-through were to fail.
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
