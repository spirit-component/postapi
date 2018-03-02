POST API
========

## build post-api

> Install spirit-builder command before you build postapi

#### Install spirit-builder command

```bash
go get github.com/go-spirit/spirit-builder
go install github.com/go-spirit/spirit-builder
```


#### Build

```bash
> spirit-builder build --config build.conf
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

components.post-api.external.grapher.driver = default

components.post-api.external.grapher.default = {

	todo-task-new {
		name  = "todo.task.new"
		graph = {
			errors {
				to-queue {
					seq = 1
					url = "spirit://actors/fbp/mns/endpoint?queue=api-call-error"
				}

				response {
					seq = 2
					url = "spirit://actors/fbp/post-api/external?action=callback"
				}
			}

			normal {
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
					url = "spirit://actors/fbp/post-api/external?action=callback"
				}
			}
		}
	}
}
```