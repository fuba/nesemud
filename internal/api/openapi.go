package api

const openAPISpec = `{
  "openapi": "3.1.0",
  "info": {
    "title": "NES Daemon API",
    "version": "0.1.0"
  },
  "paths": {
    "/v1/state": {"get": {"responses": {"200": {"description": "ok"}}}},
    "/v1/rom/load": {"post": {"responses": {"202": {"description": "accepted"}}}},
    "/v1/control/reset": {"post": {"responses": {"202": {"description": "accepted"}}}},
    "/v1/replay/fm2": {"post": {"responses": {"202": {"description": "accepted"}}}},
    "/v1/memory/{addr}": {
      "get": {"responses": {"200": {"description": "ok"}}},
      "put": {"responses": {"204": {"description": "updated"}}}
    },
    "/v1/input/player/{id}": {"put": {"responses": {"204": {"description": "updated"}}}},
    "/v1/control/pause": {"post": {"responses": {"202": {"description": "accepted"}}}},
    "/v1/control/resume": {"post": {"responses": {"202": {"description": "accepted"}}}}
  }
}`
