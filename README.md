POST API
========

## build post-api

#### create imports file

imports_postapi.go

```go
package main

import (
	_ "github.com/go-spirit/spirit/component/mns"
	_ "github.com/spirit-component/postapi"
)
```

#### copy imports file to go-spirit dir

```bash
> cp imports_postapi.go $GOPATH/src/github.com/go-spirit/go-spirit 
```

#### build

```bash
> cd $GOPATH/src/github.com/go-spirit/go-spirit
> go build -o postapi ./main.go ./imports_postapi.go
```

#### make sure the components are already registered

```bash
> ./postapi list components
- function: github.com/go-spirit/spirit/component/function.newComponentFunc
- mns: github.com/go-spirit/spirit/component/mns.NewMNSComponent
- post-api: github.com/spirit-component/postapi.NewPostAPI
```


#### run components

```
> ./postapi run --config configs/postapi.conf
```

`postapi.conf` example

```hocon

include "configs/secret.conf"

components.mns.endpoint {

	access-key-id = ${aliyun.access-key-id}
	access-key-secret = ${aliyun.access-key-secret}
	endpoint = ${aliyun.endpoint}

	queues {
		api-call-error {}
		api-call-back {}
	}
}

components.post-api.external.api = {

	todo-task-new {
		name  = "todo.task.new"
		graph = {
			errors {
				to-queue {
					seq = 1
					url = "spirit://actors/mns/endpoint?queue=api-call-error"
				}

				response {
					seq = 2
					url = "spirit://actors/post-api/external?action=callback"
				}
			}

			normal {
				to-queue-new-task {
					seq = 1
					url = "spirit://actors/mns/endpoint?queue=todo-task-new"
				}

				to-todo {
					seq = 2
					url = "spirit://actors/examples-todo/todo?action=new"
				}

				to-callback-queue {
					seq = 3
					url = "spirit://actors/mns/endpoint?queue=api-call-back"
				}

				response {
					seq = 4
					url = "spirit://actors/post-api/external?action=callback"
				}
			}
		}
	}

	todo-task-get {
		name  = "todo.task.get"
		graph = {
			errors {
				to-queue {
					seq = 1
					url = "spirit://actors/mns/endpoint?queue=api-call-error"
				}

				response {
					seq = 2
					url = "spirit://actors/post-api/external?action=callback"
				}
			}

			normal {
				to-queue-get-task {
					seq = 1
					url = "spirit://actors/mns/endpoint?queue=todo-task-get"
				}

				to-todo {
					seq = 2
					url = "spirit://actors/examples-todo/todo?action=get"
				}

				to-callback-queue {
					seq = 3
					url = "spirit://actors/mns/endpoint?queue=api-call-back"
				}

				response {
					seq = 4
					url = "spirit://actors/post-api/external?action=callback"
				}
			}
		}
	}
}
```