POST API
========

## build postapi

> Install go-spirit command before you build postapi

#### Install go-spirit command

```bash
go get github.com/go-spirit/go-spirit
go install github.com/go-spirit/go-spirit
```


#### Build

```bash
> go-spirit build --config build.conf
```


#### make sure the components are already registered

```bash
> ./postapi list components
- function: github.com/go-spirit/spirit/component/function.newComponentFunc
- mns: github.com/go-spirit/spirit/component/mns.NewMNSComponent
- postapi: github.com/spirit-component/postapi.NewPostAPI
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

components.postapi.external.grapher.driver = default

components.postapi.external.grapher.default = {

	todo-task-new {
		name  = "todo.task.new"
		graph = {
			error {
				to-queue {
					seq = 1
					url = "spirit://actors/fbp/mns/endpoint?queue=api-call-error"
				}

				response {
					seq = 2
					url = "spirit://actors/fbp/postapi/external?action=callback"
				}
			}

			entrypoint {
				to-queue-new-task {
					seq = 1
					url = "spirit://actors/fbp/mns/endpoint?queue=todo-task-new"
				}

				to-todo {
					seq = 2
					url = "spirit://actors/fbp/examples-todo/todo?action=new"
				}

				to-callback-queue {
					seq = 3
					url = "spirit://actors/fbp/mns/endpoint?queue=api-call-back"
				}

				response {
					seq = 4
					url = "spirit://actors/fbp/postapi/external?action=callback"
				}
			}
		}
	}
}
```