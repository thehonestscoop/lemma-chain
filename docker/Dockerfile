
# STEP 1: build dgraph and lemma-chain
FROM golang:1.12.1-alpine3.9 AS build

RUN apk update && apk add --no-cache --update curl git

## set environment variable that's available
## only during Image build
ARG DGRAPH_BRANCH=master

### Build Dgraph binary ###

RUN go get -d -v github.com/dgraph-io/dgraph/dgraph
RUN cd $GOPATH/src/github.com/dgraph-io/dgraph && git checkout ${DGRAPH_BRANCH}
RUN CGO_ENABLED=0 go build -v -o /dgraph github.com/dgraph-io/dgraph/dgraph

### Build Lemma-chain binary ###

## get all dependencies and build lemma-chain
RUN go get -d -v github.com/thehonestscoop/lemma-chain
Run cd $GOPATH/src/github.com/thehonestscoop/lemma-chain
RUN CGO_ENABLED=0 go build -v -o /lemma-chain github.com/thehonestscoop/lemma-chain

### Build Goofys binary ###
RUN go get -d -v github.com/kahing/goofys
RUN cd $GOPATH/src/github.com/kahing/goofys
RUN CGO_ENABLED=0 go build -v -o /goofys github.com/kahing/goofys


# STEP 2: build alpine image
FROM alpine:3.9
RUN apk update && apk add --update --no-cache \
	autoconf automake bash build-base curl \
	curl-dev fuse-dev git \
	libxml2-dev python pkgconf syslog-ng 


# Download, Install and setup aws-cli
RUN mkdir /temp && cd /temp && \
	curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip" && \
	unzip awscli-bundle.zip && \
	./awscli-bundle/install -i /usr/local/aws -b /usr/local/bin/aws && \
	rm -rf /temp

COPY --from=build /dgraph /usr/local/bin/dgraph
COPY --from=build /lemma-chain /usr/local/bin/lemma-chain
COPY --from=build /goofys /usr/local/bin/goofys
# COPY --from=build /usr/local/bin/s3fs /usr/local/bin/s3fs
### Build s3fs-fuse binary
RUN cd /tmp && \
	git clone --depth 1 https://github.com/s3fs-fuse/s3fs-fuse.git && \
    cd s3fs-fuse && ./autogen.sh && ./configure && make -j4 > /dev/null && make install && \
    cd .. && rm -Rf s3fs-fuse

RUN mkdir /dgraph
WORKDIR /dgraph

EXPOSE 1323

# Set environment for AWS_ACCESS variables. 
ENV AWS_ACCESS_KEY_ID= \
	AWS_SECRET_ACCESS_KEY= \
	BUCKET_NAME=

COPY dgraph_entry.sh /dgraph_entry.sh
RUN chmod +x /dgraph_entry.sh

CMD /dgraph_entry.sh
