package postapi

import (
	"github.com/gogap/config"
	"testing"
)

var (
	testConfStr = `

todo-task-add {
	name  = "todo.task.add"
	graph = {
		
		errors {
			to-queue {
				url      = "spirit://actors/mns/sender?queue=post-api-error"
				metadata = {}
			}
		}

		ports {
			to-queue {
				url      = "spirit://actors/mns/sender?queue=todo-task-add"
				metadata = {}
			}

			add-task {
				url      = "spirit://actors/example-todo/todo?action=add"
				metadata = {}
			}

			callback {
				url      = "spirit://actors/mns/sender?queue=post-api-callback"
				metadata = {}
			}
		}
	}
}
`
)

func TestDefaultGraphProviderLoad(t *testing.T) {

	conf := config.NewConfig(config.ConfigString(testConfStr))

	graphProvider, err := newDefaultGraphProvider(conf)

	if err != nil {
		t.Fatal("laod grahp failure:", err.Error())
		return
	}

	g, exist := graphProvider.Query("todo.task.add")

	if !exist {
		t.Fatal("could not find api todo.task.add")
		return
	}

	if len(g.Errors) != 1 {
		t.Fatal("errors ports count error")
		return
	}

	if len(g.Ports) != 3 {
		t.Fatal("ports count error")
		return
	}

}
