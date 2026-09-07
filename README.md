<h1 align="center">concurrently-safe web crawler for link traversal.</h1>
<br>

<h2> features: </h2>
<h4>-few allocations/bytes per operation (see benchmarks/results.txt)</h4>
<h4>-fast, about <4000 nanoseconds time at each crawl. (see benchmarks/results.txt)</h4>
<h4>-retries failed attempts on hosts based on transient or non-transient errors.</h4>
<h4>-configurable options such as maximum depth the crawler is allowed to traverse through, and whether it can traverse external/non-host related domains.</h4>
<h1 align="center">instillation:</h1>
<h2 align="center">(for code usage):</h2>
<h1 align="center"><code align="center"> go get github.com/72sevenzy2/web-crawler </code></h1>
<br>
<h2 align="center">(for cli usage):</h2>
<h1 align="center"><code> git clone https://github.com/72sevenzy2/web-crawler.git </code></h1>
<br>
<h1 align="center">getting started:</h1>
  
  ```
  package main

  import (
      "context"
      "time"
        "github.com/72sevenzy2/web-crawler"
  )

  func main() {
    wc := crawler.NewCrawler(10, true, 2)
    /*
      NewCrawler() takes in a max depth, which is the number of times to traverse through links, and a boolean to which allow traversing through external domains other than the host, and finally, a max retry limit on whether how many times a failed request should be retried.
    */

    ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
    defer cancel()

    wc.Start(ctx, "https://jsonplaceholder.typicode.com/guide/")
    /*
      wc.Start() takes in a context representing the crawlers lifetime, and a url to scour.
    */
  }
  ```

  <br>
<h1 align="center">and to use the cli:</h1>

```
cd web-crawler
cd cli

go run main.go [-depth <int> -retries <int> -cross-domains <bool>]

# the defaults are set as:
# depth : 10
# retries : 5
# cross-domains : true

# depth represents how deep the crawler is to crawl through while traversing a link, retries represent how many times the crawler is to-
# request a sub-domain within the host if it were to fail, and cross-domains allow the crawler to traverse through external/non-host domains.
```
