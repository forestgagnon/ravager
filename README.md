# ravager -- a simple HTTP load generator

It makes a ton of requests. It also tries not to suck.

Basic load generating functionality behind a simple CLI. No byzantine config files or complex setup.

The output gives you counts of each status code, and the effective requests-per-second (`rps`) while it is running. The logs are structured JSON.

## Usage

### Docker
https://hub.docker.com/r/forestgagnon/ravager

You may need to pass in a special ulimit at high parallelism

`docker run --rm -it forestgagnon/ravager --help`

```bash
docker run --ulimit nofile=20000:20000 --rm -i forestgagnon/ravager \
  --url http://mysite.mysite \
  --method POST \
  --parallelism 1000 \
  --numrequests 20000 \
  --header "Authorization:Bearer foo" \
  --header "Content-Type:application/json" \
  --body '{"foo":"bar"}'
```
### Go toolchain

Watch out for `ulimit` on number of files. `ulimit -n` sets it for your shell. e.g. if you use parallelism of `1000`, you should make sure the ulimit on number of open files is high enough, like `ulimit -n 2000`

```
go install github.com/forestgagnon/ravager@latest
```
