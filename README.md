# pizza
This is an engine that sources git commits and turns them to insights. Ideally this will be useful for any open source maintainer trying to get insight into their open source. 

<img width="1350" alt="Screen Shot 2023-05-11 at 8 46 18 AM" src="https://github.com/open-sauced/pizza/assets/5713670/b91989d8-df6d-4631-8d7d-3089b76ee113">

## Scope
Build a docker container clones a repo and stores the contributors and their contributions (commits) in a relational tables. This data should be store in a postgres database (😃). 

## Bonus
- Make this work with orchestration that fetches the latest data on a cron every hour.
- Add a queue to assist in fetch content without rate limiting.
- Add the ability to fetch all repos in an org.
- Visualize this data somehow.

## Gotchas
- Large repos like k8s or linux will trip `git clone` the rate limiter if done multiple times in an hour. How would you account for fetching large repos with lots of data?

## 🚀 Routes

### `/bake`

The bake route accepts a `POST` request with the following json body:

```json
{
    "url": "https://some-git-repo.com"
}
```

The server will then process the commits in the provided git repository by
cloning the repo into memory, storing the individual commits and their authors.

Example:

```bash
curl -d '{"url":"https://github.com/open-sauced/insights"}' \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/bake
```


## 🖥️ Local development

There are a few required dependencies to build and run the pizza-oven service:

- The [Go toolchain](https://go.dev/doc/install)
- [Docker](https://docs.docker.com/engine/install/) (for building the container)
- Make

### Local postgres database setup

You can use a local postgres database (like with `brew services start postgresql`)
[configured to accept SSL connections](https://www.postgresql.org/docs/current/ssl-tcp.html),
an `.env` file with the database's secrets,
and a locally running version of the Go application.

To start the pizza oven service:

```
$ make run
{"level":"info","ts":1687800220.829255,"caller":"server/server.go:36","msg":"initiated zap logger with level: 0"}                                          │
{"level":"info","ts":1687800220.8293574,"caller":"server/server.go:48","msg":"Starting server on port 8080"}
```

This will start the go app, connect to your local postgres database
using your `.env` file or existing environment variables,
and start accepting requests.

### Local kubernetes setup

To get a local environment setup with a postgres database without having to start and configure one yourself,
there is a convenience script that can be invoked with `make setup-test-env` which will bootstrap:
- A local kind kubernetes cluster
- A postgres database configured with the migrations using a postgres operator
- Build the pizza-oven container
- Load the newly built image to the kind cluster
- Start a deployment for the pizza-oven service

In order to run this setup script, you will _also_ need:
- Kubectl
- helm
- Kind
- The psql command line tool

Once you see the final step complete, the script will open a port to the pizza-oven service
and be able to start accepting requests.

