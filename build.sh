#!/bin/bash

oapi-codegen \
    --package=api \
    --generate types,server \
    <( yq \
        --input-format json \
        --output-format yaml \
        <( curl --silent http://localhost:8080/v3/api-docs ) \
    ) \
    > src/api.gen.go
