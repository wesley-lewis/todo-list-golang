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
	hostName 				string = "localhost:27017"			// go will connect to mongodb on this host
	dbName 					string = "demo_todo"						// the database name in mongodb
	collectionName 	string = "todo"									// the collection name withing the database "demo_todo"
	port 						string = ":9000"								// the port number on which the server will be running to accept incoming client requests
)	


// The struct tag is used for specifying the field name in the BSON document that corresponds to the struct field. 
// For eg in the below todoModel struct: The 'Title' in the struct will be mapped to 'title' in the bson document 
// omitempty will not marshal the ID meaning if the ID field is empty then it won't be converted to the bson format
type (
	todoModel struct {
		ID							bson.ObjectId	`bson:"_id,omitempty"`		
		Title						string 				`bson:"title"`
		Completed				bool					`bson:"completed"`
		CreatedAt				time.Time			`bson:"created_at"`
	}

	todo struct {
		ID							string 			`json:"id"`
		Title						string 			`json:"title"`
		Completed				bool				`json:"completed"`
		CreatedAt				time.Time		`json:"created_at"`
	}
)

// This function will execute before the main() function initializing the variables and connecting to the database
// You would notice that this function isn't called anywhere yet it runs first. This is the power of the init function.
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

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo 		// for communcating with the frontend we require a JSON object hence we use todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	// validation of title
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title field is important",
		})
		return
	}

	// if the input is correct, we will create a todo
	// for communicating with the database we will require a BSON object. Hence we will be creating a todoModel
	tm := todoModel{
		ID: 				bson.NewObjectId(),
		Title: 			t.Title,
		Completed:	false,
		CreatedAt: time.Now(),
	}

	if err := db.C(collectionName).Insert(&tm); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to save todo",
			"error": err,
		})
		return
	}

	// if everything is done properly
	rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "Todo created successfully",
		"todo_id": tm.ID.Hex(),
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	// chi.URLParam is used to retrive values from the url 
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	// checks if the id supplied via the url is an valid ObjectId in Hex format
	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The id is invalid",
		})
		return
	}

	var t todo
	
	if err:=json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	// validation of the title field
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title field is required",
		})
		return
	}

	// if input is okay, update the todo
	// We provide bson.M{} as it can communicate with the database. 
	if err:=db.C(collectionName).
					Update(
						bson.M{"_id": bson.ObjectIdHex(id)}, // this value is used to select the object in the database
						bson.M{"title": t.Title, "completed":t.Completed},	// this value is used to update the actual value in the database
					); err != nil {
						rnd.JSON(w, http.StatusProcessing, renderer.M{
							"message": "Failed to update todo",
							"error": err,
						})
						return
					}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo updated successfully",
	})
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
	// create a slice of todoModel as there could be a number of tasks within the database
	todos := []todoModel{}
	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to fetch todo",
			"error": err,
		})
		return
	}
	// Create a todoList of type todo as it will be used to communicate with the frontend as it is in JSON format. 
	todoList := []todo{}
	for _, t := range todos {
		todoList = append(todoList, todo{
			ID: 			t.ID.Hex(),
			Title: 		t.Title,
			Completed:t.Completed,
			CreatedAt:t.CreatedAt,
		})
	}

	// the data is sent as json 
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	// the ID field will be used to find that particular record within the database and delete it
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The id is invalid",
		})
		return
	}

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

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)

	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr: 					port,
		Handler:				r,
		ReadTimeout: 		60 * time.Second,
		WriteTimeout:		60 * time.Second,
		IdleTimeout: 		60 * time.Second,
	}

	// Creating a new goroutine for accepting requests
	go func() {
		log.Println("Listening on port", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Listen: %s\n", err)
		}
	}()

	<- stopChan
	log.Println("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("Server gracefully stopped")
}

// todoHandler will map the incoming requests to appropriate function to handle that type of request
// it is grouped together as they will all have a common path of "http://localhost:9000/todo/"
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
		log.Fatal("Error:", err)
	}
}



// REFERENCES
// 1) https://www.mongodb.com/docs/drivers/go/current/fundamentals/bson/#std-label-golang-bson