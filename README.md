<div align="center">
  <br>
  <img alt="Open Sauced" src="https://i.ibb.co/7jPXt0Z/logo1-92f1a87f.png" width="300px">
  <h1>🍕 Pizza Oven Micro-service 🍕</h1>
  <strong>A Go micro-service that sources git commits from any arbitrary git repo and indexes them into a postgres database.</strong>
  <br>
</div>
<br>
<p align="center">
  <img src="https://img.shields.io/github/languages/code-size/open-sauced/pizza" alt="GitHub code size in bytes">
  <a href="https://github.com/open-sauced/pizza/issues">
    <img src="https://img.shields.io/github/issues/open-sauced/pizza" alt="GitHub issues">
  </a>
  <a href="https://github.com/open-sauced/api.opensauced.pizza/releases">
    <img src="https://img.shields.io/github/v/release/open-sauced/pizza.svg?style=flat" alt="GitHub Release">
  </a>
  <a href="https://discord.gg/U2peSNf23P">
    <img src="https://img.shields.io/discord/714698561081704529.svg?label=&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2" alt="Discord">
  </a>
  <a href="https://twitter.com/saucedopen">
    <img src="https://img.shields.io/twitter/follow/saucedopen?label=Follow&style=social" alt="Twitter">
  </a>
</p>

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
[configured to accept SSL connections](https://www.postgresql.org/docs/current/ssl-tcp.html)
that has been bootstrapped with the [OpenSauced API migrations](https://github.com/open-sauced/api/tree/beta/migrations).
It is highly recommended to follow the instructions in the API repository to get a locally running postgres going
that can be used with the `pizza` oven micro-service.

You'll also need an `.env` file with the database's secrets
(see `.env.example` in this repo for the required env variables),
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

See the `.env.example` file to see what environment variables are expected.

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
- Helm
- Kind
- The psql command line tool

Once you see the final step complete, the script will open a port to the pizza-oven service
and be able to start accepting requests.

