# brightwave-google

Google API Supporting Index and Search Methods

## Build Instructions

### Prerequisites

- A working Go environment 1.22 or greater
- [Install](https://buf.build/docs/installation) the `buf` cli
- [Install](https://docs.sqlc.dev/en/stable/overview/install.html) `sqlc`

### Code Generation

From the root of the repo. Note that the generated files are committed and kept up-to-date.

```sh
buf generate
sqlc generate
```

### Building

```sh
go build ./cmd/google
```

## Using the google command

### Starting the server

```sh
## see available flags with ./google serve -h
./google serve --num-crawlers 10
```

### Indexing Pages

```sh
./google index https://www.cnn.com 1 # where 1 is the depth
```

### Searching

The search algorithm finds all pages with all matching terms and then sorts them by number of matches, relevance, and number of origins, importance. In practice, this seems to work tolerably, but not amazingly well.

```sh
./google search breaking news
```

### Limitations

1. Only UTF-8 encoded text can be properly processed
2. English is the only language that can be properly tokenized and lemmatized
3. Webpages are not browser rendered, so javascript content can not be indexed
4. All testing was done by hand, unit tests are desperately needed
5. SQLite is a decent choice for a datastore, but it can't handle concurrent writers so a mutex had to be used to prevent errors saying that the database was in use

### Justification for Liberties Taken

The assignment asked that I limit the use of 3rd party libraries. I did use some though and I felt that I should explain why.

1. Anything under `golang.org/x` is generally considered 1st party as it is developed by Google. I used that code for text normalization and transformation as well as html parsing/tokenization.
2. `github.com/spf13/cobra` is a common library to help build cli applications
3. `google.golang.org/grpc` and `google.golang.org/protobuf` are required to build the gRPC API
4. `github.com/mattn/go-sqlite3` is required to interact with the SQLite database
5. `github.com/hashicorp/go-cleanhttp` helps to ensure that I'm using a properly configured, pooled, http transport
6. `github.com/aaaton/golem` is perhaps a bit controversial as I use it to lemmatize the words after being tokenized. I'm not sure that it was strictly needed as the search isn't amazing, but I had hoped that it would improve the search to a degree. It also imposes a language restriction, English. Overall, I probably could have done without it. I left it in because it was neat.
7. `github.com/jdkato/prose/v2` is also a bit controversial as I use it to extract the text tokens from the documents as well as remove parts of speach that are not particularly useful to index like infinitives and conjunctions. Go has excellent support for unicode owing to the fact that one of its inventors, Rob Pike, was also the inventor of UTF-8. I could have iterated over the runes in the byte slice and extracted tokens according to something like the [Unicode Text Segmentation Standard](https://www.unicode.org/reports/tr29/). It wouldn't have been too hard to implement, but I wouldn't have been able to determine the type of word each token was. In order to limit the time spent on this, I chose to use a library.

In all, I felt these were reasonable choices to make as the core concerns of the assignment were implemented without libraries. Those include crawling, redirect following, depth tracking, building a reverse index and designing a reasonable search algorithm.

### Next Steps

I would definitely move to a real queue/stream product. Managing a durable, highly available set of URLs to be indexed would make scaling this much easier. I'd probably use something like kafka, but there are other choices that could be considered depending on the expected load. With that in place, I would front the system with an API Gateway routing indexing requests to a simple api frontend to put the request on kafka. From there, "fetcher" workers would be employed to download the URL data. This data should eventually go into something like elasticsearch. The number of fetchers and elasticsearch (to be able to ingest and index fast enough to keep up) need to be scaled proportionately.

For the search side, I'd make another service that the API Gateway routes to. It would be fairly simple and just need to query elasticsearch.

The elasticsearch configuration is central to everything in this design. It should have preprocess pipelines to handle language detection and stemming (lemmatizing would be preferred, but I couldn't find it in the docs).

A mechanism to keep track of all the origins for a particular page is also necessary, either in elasticsearch or a separate database. This is the way page "importance" is calculated (also known as page rank). An out-of-band process should be run periodically to recalculate page rank, from 0 to 1, of every page in the index. Search results should be ordered by relevance and importance.
