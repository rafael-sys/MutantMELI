package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/* Variable para singleton client, inicializada en  GetMongoClient() */
var clientInstance *mongo.Client

//Usado mientras se crea el singleton en GetMongoClient().
var clientInstanceError error

//Usado para creación del cliente
var mongoOnce sync.Once

//Constantes de acceso a BD
const (
	CONNECTIONSTRING = "mongodb+srv://meli:Meli*2021@cluster0.l8sdz.mongodb.net/myFirstDatabase?retryWrites=true&w=majority"
	DB               = "meli"
	ISSUES           = "meli"
)

//Metodo encargado obtener el cliente de MongoDB
func GetMongoClient() (*mongo.Client, error) {

	mongoOnce.Do(func() {

		clientOptions := options.Client().ApplyURI(CONNECTIONSTRING)

		client, err := mongo.Connect(context.TODO(), clientOptions)
		if err != nil {
			clientInstanceError = err
		}

		err = client.Ping(context.TODO(), nil)
		if err != nil {
			clientInstanceError = err
		}
		clientInstance = client
	})

	return clientInstance, clientInstanceError
}

//Estructura usada para recibir los datos por el API
type mutant struct {
	DNA      []string `json.dna`
	IsMutant bool     `json.isMutant`
}

//Estructura para retornar las estadisticas en el metodo /stats
type mutantStadistics struct {
	Count_mutant_dna int     `json.count_mutant_dna`
	Count_human_dna  int     `json.count_human_dna`
	Ratio            float64 `json.ratio`
}

//Estructura usada para obtener la información de MongoDB
type mutantDB struct {
	DNA      []string `bson: "dna"`
	IsMutant bool     `bson: "ismutant"`
}

//Metodo encargado de validar si es mutante a partir de un array de string
func isMutant(dna []string) (bool, error) {

	totalDna := 0

	for i := 0; i < len(dna); i++ {

		if len(dna[i]) == 6 {
			for j := 0; j < len(dna[i]); j++ {

				//Horizontal
				if j < len(dna[i])-3 {
					if isEqual([]rune(dna[i])[j], []rune(dna[i])[j+1], []rune(dna[i])[j+2], []rune(dna[i])[j+3]) {
						totalDna++
					}
				}

				//Vertical
				if i < len(dna)-3 {
					if isEqual([]rune(dna[i])[j], []rune(dna[i+1])[j], []rune(dna[i+2])[j], []rune(dna[i+3])[j]) {
						totalDna++
					}
				}

				//Diagonal
				if i < len(dna)-3 && j < len(dna[i])-3 {
					if isEqual([]rune(dna[i])[j], []rune(dna[i+1])[j+1], []rune(dna[i+2])[j+2], []rune(dna[i+3])[j+3]) {
						totalDna++
					}
				}

				//Diagonal invertido
				if i >= 3 && j < len(dna[i])-3 {
					if isEqual([]rune(dna[i])[j], []rune(dna[i-1])[j+1], []rune(dna[i-2])[j+2], []rune(dna[i-3])[j+3]) {
						totalDna++
					}
				}

				if totalDna > 1 {
					return true, nil
				}

			}
		} else {
			return false, errors.New("La cadena " + dna[i] + " tiene una logitud diferente de 6 caracteres.")
		}
	}

	return false, nil
}

//Metodo encargado de validar si se cumple la secuencia de DNA
func isEqual(a rune, b rune, c rune, d rune) bool {
	return a == b && b == c && c == d
}

//Metodo index del API
func indexRoute(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to my API.")
}

//Metodo ejecutado a traves de la ruta /mutant
func validateMutant(w http.ResponseWriter, r *http.Request) {
	var mutantObject mutant

	reqBody, err := ioutil.ReadAll(r.Body)

	if err != nil {
		fmt.Fprintf(w, "Insert a valid array string")
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(reqBody), &result)
	json.Unmarshal(reqBody, &mutantObject)

	validMutant := false
	mutanteDB := validateMutantDB(mutantObject)

	if len(mutanteDB) > 0 {
		validMutant = mutanteDB[0].IsMutant
	} else {
		validMutant, err = isMutant(mutantObject.DNA)

		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(err.Error())
			return
		} else {
			mutantObject.IsMutant = validMutant
			createMutantDB(mutantObject)
		}
	}

	if validMutant {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
	}

	json.NewEncoder(w).Encode(validMutant)
}

//Metodo encargado de crear un mutante en base de datos
func createMutantDB(prmMutant mutant) bool {

	client, err := GetMongoClient()
	if err != nil {
		return false
	}

	collection := client.Database(DB).Collection(ISSUES)

	_, err = collection.InsertOne(context.TODO(), mutantDB{DNA: prmMutant.DNA, IsMutant: prmMutant.IsMutant == true})
	if err != nil {
		return false
	}

	return true
}

//Metodo encargado de validar si un mutante existe en BD
func validateMutantDB(prmMutant mutant) []mutantDB {

	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database(DB).Collection(ISSUES)

	filterCursor, err := collection.Find(context.TODO(), bson.M{"dna": arrayStringToBson(prmMutant.DNA)})

	if err != nil {
		log.Fatal(err)
	}

	var mutantes []mutantDB

	if err = filterCursor.All(context.TODO(), &mutantes); err != nil {
		log.Fatal(err)
	}

	return mutantes
}

//Metodo encargado de convertir un array de string a un bson array
func arrayStringToBson(stringArray []string) bson.A {
	result := bson.A{}

	for i := 0; i < len(stringArray); i++ {
		result = append(result, stringArray[i])
	}

	return result
}

//Metodo ejecutado a traves de la ruta /stats, encargado de sacar las estadisticas
func getStadisticsMutant(w http.ResponseWriter, r *http.Request) {

	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database(DB).Collection(ISSUES)

	filterCursor, err := collection.Find(context.TODO(), bson.M{})

	if err != nil {
		log.Fatal(err)
	}

	var mutantes []mutantDB

	if err = filterCursor.All(context.TODO(), &mutantes); err != nil {
		log.Fatal(err)
	}

	stadistics := mutantStadistics{}

	for _, mutant := range mutantes {

		if isMutantStruct(mutant) {
			stadistics.Count_mutant_dna++
		} else {
			stadistics.Count_human_dna++
		}
	}

	stadistics.Ratio = float64(stadistics.Count_mutant_dna) / float64(stadistics.Count_human_dna)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stadistics)
}

//Metodo encargado de validar si un registro guardado en BD es mutante o no
func isMutantStruct(mutant mutantDB) bool {
	return mutant.IsMutant
}

func main() {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", indexRoute)
	router.HandleFunc("/mutant", validateMutant).Methods("POST")
	router.HandleFunc("/stats", getStadisticsMutant).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}
