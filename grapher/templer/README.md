`postapi.conf`

```hocon
# .....
# .....
components.postapi.external.grapher.driver = templer

components.postapi.external.grapher.templer = {

	default {
		template = "graph.json"
	}

	todo-task-new {
		name  = "todo.task.new"
		template = "todo.task.new.json"
	}
}
```

`graph.json`

```
{
    "error": {
        "ports": [{
            "seq": 1,
            "url": "spirit://actors/fbp/postapi/external?action=callback"
        }]
    },
    "entrypoint": {
        "ports": [{
            "seq": 1,
            "url": "spirit://actors/fbp/goja/api-mock?action={{.api}}"
        },{
            "seq": 2,
            "url": "spirit://actors/fbp/postapi/external?action=callback"
        }]
    }
}
```