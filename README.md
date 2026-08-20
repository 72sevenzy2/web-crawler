<h1 align="center">a minimal web crawler for traversing/link analysis.</h1>
<br>
<h2>usage:</h2>
  
  ```
  package main

  import (
    	"github.com/72sevenzy2/web-crawler"
  )

  func main() {
    wc := crawler.NewCrawler(10) // passing in a max depth (number of times to traverse links).

    wc.Start("https://jsonplaceholder.typicode.com/guide/", 0)
    /*
      wc.Start() takes in a url, and a start depth in which the crawler is to start from, so each time it scours for links in given url,
      it gradually increments start depth untill it has exceeded the max depth.
    */
  }
  ```
