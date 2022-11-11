package main

import (
	"encoding/json"
	"log" // to log the errors
	"net/http" // will help to create server in golang
	"strings" 
	"time" // timestamp to put in the database
	"context"
	"os"
	"os/signal"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var rnd *renderer.Render
var db *mgo.Database

const (
	hostName				string = "localhost:27017"
	dbName					string = "demo_todo"
	collectionName	string = "todo"
	port 						string = ":9000"
)

type(
	todoModel struct{
		ID							bson.ObjectId `bson:"_id,omitempty"`
		Title 					string 				`bson:"title"`
		Completed 			bool 					`bson:"completed"` 
		CreatedAt 			time.Time 		`bson:"createdAt"`
	}

	todo struct {
		ID					string 		`json:"id"`
		Title 			string 		`json:"title"`
		Completed		bool 			`json:"completed"`
		CreatedAt		time.Time `json:"created_at"`
	}
)

func init() {
	rnd = renderer.New()
	sess, err := mgo.Dial(hostName)
	checkErr(err)
	sess.SetMode(mgo.Monotonic, true)
	db = sess.DB(dbName)

}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkErr(err)
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
	todos := []todoModel{}

	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to fetch todo",
			"error": err,
		})
		return
	}
	todoList := []todo{}

	for _, t := range todos {
		todoList = append(todoList, todo{
			ID: 				t.ID.Hex(),
			Title: 			t.Title,
			Completed: 	t.Completed,
			CreatedAt: 	t.CreatedAt,
		})
	}
	
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}


// 5 steps inside this function
// step 1: decode the body of the request received from the user
// step 2: validation; the request that the user has sent is there or not
// step 3: create a todo model to send that model to the database
// step 4: send the model to the database
// step 5: send the response to the user that the model has been created succesffully
func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo

	// step 1
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return 
	}

	// step 2
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title is required",
		})
		return 
	}

	// step 3
	tm := todoModel{
		ID: bson.NewObjectId(),
		Title: t.Title,
		Completed: false,
		CreatedAt: time.Now(),
	}

	// step 4
	if err := db.C(collectionName).Insert(&tm); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to save todo",
			"error": err,
		})
		return 
	}

	// step 5
	rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "Todo created successfully",
		"todo_id": tm.ID.Hex(),
	})
}

// 4 steps
// step 1: work with the id which is passed using the http request
// step 2: the id that has been sent is hex or not
// step 3: working with the db and removing the particular record with the ID
// step 4: return the message to the frontend that the record has been deleted
func deleteTodo(w http.ResponseWriter, r *http.Request) {
	// step 1
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	
	// step 2
	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The ID is invalid",
		})
		return 
	}
	// step 3
	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to delete todo", 
			"error": err,
		})
		return 
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})
}


func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The ID is invalid",
		})
		return
	}

	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if t.Title == "" {
	rnd.JSON(w, http.StatusBadRequest, renderer.M{
		"message": "The Title field is required",
	})
	return
	}

	if err := db.C(collectionName).Update(
		bson.M{"_id": bson.ObjectIdHex(id)},
		bson.M{"title":t.Title, 
		"completed": t.Completed},
	); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to update todo",
		})
		return 
	}



}



func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr: port, 
		Handler: r,
		ReadTimeout: 60*time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Println("Listening on port", port)
		if err:= srv.ListenAndServe(); err != nil {
			log.Printf("Listen:%s\n", err)
		}
	}()

	<- stopChan
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("Server gracefully stopped")
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}