# ReMesh

CLI-tool that converts Core Mesh `Mesh` custom resources into Gateway API manifests (currently HTTPRoute) and optionally validates them with bundled CRD schemas.

## Features
- Walks through a directory, preprocessing Helm-templated YAML fragments and splitting multi-document files.
- Routes `Mesh` fragments by `subKind` to pluggable handlers (via Java `ServiceLoader`) and produces Gateway API resources.
- Validates generated resources against CRD schemas shipped under `src/main/resources/schemas`.

## Build
```bash
mvn clean package
```

## Run
After packaging, run the shaded JAR:
```bash
java -jar target/remesh-1.0.0.jar -d ./configs -v
```
Options:
- `-d, --dir` — directory with YAML files (defaults to current directory).
- `-v, --validate` — enable CRD validation for generated resources.
