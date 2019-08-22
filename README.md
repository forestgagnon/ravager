# ravager

This is a prototype work in progress. It makes a ton of requests. It also tries not to suck.

The output gives you counts of each status code, and the requests-per-second (`rps`) while it is running.

## Usage

https://hub.docker.com/r/forestgagnon/ravager

You may need to pass in a special ulimit at high parallelism

`docker run --rm -it forestgagnon/ravager --help`

```bash
docker run --ulimit nofile=20000:20000 --rm -it forestgagnon/ravager \
  --url http://mysite.mysite \
  --method POST \
  --parallelism 1000 \
  --numrequests 20000 \
  --header "Authorization:Bearer foo" \
  --header "Content-Type:application/json" \
  --body '{"foo":"bar"}'
```
