`postapi.conf`

```hocon
# .....
# .....
components.post-api.external.grapher.driver = templer

components.post-api.external.grapher.templer = {

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
            "url": "spirit://actors/fbp/post-api/external?action=callback"
        }]
    },
    "entrypoint": {
        "ports": [{
            "seq": 1,
            "url": "spirit://actors/fbp/goja/api-mock?action={{.api}}"
        },{
            "seq": 2,
            "url": "spirit://actors/fbp/post-api/external?action=callback"
        }]
    }
}
```