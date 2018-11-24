# Coordinator

## Running locally

Run

    go install golang.org/x/build/cmd/coordinator && coordinator --mode=dev --env=dev

to start a server on https://localhost:8119. Proceed past the TLS warning and
you should get the homepage. Some features won't work when running locally,
but you should be able to view the homepage and the builders page and do basic
sanity checks.

#### Render the "Trybot Status" page locally

To view/modify the "Trybot Status" page locally, you can build the coordinator
with the `-dev` tag.

    go install -tags=dev golang.org/x/build/cmd/coordinator

Then start the coordinator and visit https://localhost:8119/try-dev in your
browser. You should see a trybot status page with some example data.
