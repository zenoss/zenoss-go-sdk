# zenoss-go-sdk

Zenoss Go SDK.

## TODOs

- Documentation.
- Consider configuration compatibility with Node.js, Python, etc.
- Support structured logging without specific logging dependency.
- Instrument with OpenCensus stats. (finish) 
- Use in zenoss-agent-kubernetes.
- Use in prometheus-adapter-zenoss.
- Compact metrics.

- Matching
    - regular expression (various)
    - in-field (on metadataField list values)

- Actions
    - log (improve: choose fields, format, etc.)
    - default-tag
    - default-tags
    - default-dimension
    - default-dimensions
    - default-metadata-field
    - default-metadata-fields
    - set-tag
    - set-tags
    - set-dimension
    - set-dimensions
    - set-metadata-field
    - set-metadata-fields
    - set-metric
    - scale-value

## Documentation

- Configuration
    - Mechanisms
    - Programmatic
    - YAML
    - Templates
        - Template language is Mustache.
        - Use "_" instead of "." in template variable names.

- Modules
    - Endpoint
    - Processor
    - Splitter
    - Proxy

- Logging
    - Global (Level, Fields, Func)
    - Module (Level, Fields, Func)

- OpenCensus Stats

