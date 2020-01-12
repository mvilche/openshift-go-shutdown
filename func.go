package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/// define modelo en mongo
type Microservicios struct {
	Microservicio         string `bson:"Microservicio"`
	Replicas              string `bson:"Replicas"`
	Porcentaje            int64  `bson:"Porcentaje"`
	ReplicasConPorcentaje int64  `bson:"ReplicasConPorcentaje"`
	Date                  string `bson:"Date"`
	Hora                  string `bson:"Hora"`
	Usuario               string `bson:"Usuario"`
	Namespace             string `bson:"Namespace"`
}

/// define estructura en datasource
type Config struct {
	Dbpassword         string
	DbDatabase         string
	DbUser             string
	DbPort             int64
	DbHost             string
	OpenshiftHost      string
	OpenshiftUser      string
	OpenshiftPassword  string
	OpenshiftNamespace string
	OpenshiftNodos     []string
	Porcentaje         int64
	DbCollection       string
}

/// lee datasource

func empty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func ReadConfig() Config {

	if _, err := os.Stat("libs"); os.IsNotExist(err) {
		log.Fatal("ERROR NO SE ENCONTRO CARPETA libs")
		os.Exit(1)
	}

	if _, err := os.Stat("libs/oc"); os.IsNotExist(err) {
		log.Fatal("ERROR NO SE ENCONTRO BINARIO oc EN CARPETA libs \n DESCARGUE https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz")
		os.Exit(1)
	}

	if _, err := os.Stat("config"); os.IsNotExist(err) {
		log.Fatal("ERROR NO SE ENCONTRO CARPETA config")
		os.Exit(1)
	}

	var configfile = "config/config.conf"
	_, err := os.Stat(configfile)
	if err != nil {
		log.Fatal("ERROR NO SE ENCONTRO DATASOURCE: ", configfile)
		os.Exit(1)
	}

	var config Config
	if _, err := toml.DecodeFile(configfile, &config); err != nil {
		log.Fatal("ERROR AL COMPROBAR EL ARCHIVO - CONTIENE ERRORES", err)
	}

	if empty(config.DbDatabase) {
		log.Fatal("ERROR dbdatabase vacio en config.conf")
		os.Exit(1)
	}

	if empty(config.DbHost) {
		log.Fatal("ERROR dbhost vacio en config.conf")
		os.Exit(1)
	}

	s := strconv.FormatInt(int64(config.DbPort), 10)

	if empty(s) {
		log.Fatal("ERROR dbport vacio en config.conf")
		os.Exit(1)
	}

	if empty(config.OpenshiftHost) {
		log.Fatal("ERROR openshifthost vacio en config.conf")
		os.Exit(1)

	}

	if empty(config.OpenshiftUser) {
		log.Fatal("ERROR openshiftuser vacio en config.conf")
		os.Exit(1)

	}

	if empty(config.OpenshiftPassword) {
		log.Fatal("ERROR openshiftpassword vacio en config.conf")
		os.Exit(1)

	}

	if empty(config.OpenshiftNamespace) {
		log.Fatal("ERROR openshiftnamespace vacio en config.conf")
		os.Exit(1)

	}

	if len(config.OpenshiftNodos) <= 0 {
		log.Fatal("ERROR openshiftnodos vacio en config.conf")
		os.Exit(1)

	}

	return config
}

//// comprueba mongo
func Mongoconnect() {

	host := ReadConfig().DbHost
	port := ReadConfig().DbPort
	clientOpts := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%d", host, port))
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		fmt.Println("ERROR CONECTANDO A MONGO: ")
		log.Fatal(err)
		os.Exit(1)
	}

	// Check the connections
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		fmt.Println("ERROR CONECTANDO A MONGO: ")
		log.Fatal(err)
		os.Exit(1)
	}

	err = client.Disconnect(context.TODO())

	if err != nil {
		log.Fatal(err)
	}

}

func CalculaPorcentaje(replica int64) int64 {

	porcentaje := ReadConfig().Porcentaje

	var total = replica * porcentaje / 100

	/// si el porcentaje retorna 0 pero replicas es mayor a cero, retorno 1

	if total == 0 && replica > 0 {

		total = 1
	}

	/// si replica ya es cero retorno cero
	if replica == 0 {

		total = 0
	}

	return total
}

func OpenshiftGetReplicas() {

	namespace := ReadConfig().OpenshiftNamespace
	porcentaje := ReadConfig().Porcentaje
	usuario := ReadConfig().OpenshiftUser

	cmd := exec.Command(`bash`, `-c`, `libs/oc get dc -o=jsonpath='{range .items[*]}{.metadata.name}{"#"}{.status.replicas}{"\n"}{end}' -n `+namespace)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil {
		out2 := string([]byte(out))
		fmt.Println(out2)
		log.Fatalf("FALLO AL OBTENER EL NODO EN OPENSHIFT %s\n", err)
		fmt.Println("********************************************")
		os.Exit(1)
	}

	//// capturo la salida  y almaceno en slice
	r := bytes.NewReader(out)
	buff := bufio.NewScanner(r)
	var allmicros []string

	for buff.Scan() {
		allmicros = append(allmicros, buff.Text()+"\n")
	}

	host := ReadConfig().DbHost
	database := ReadConfig().DbDatabase
	port := ReadConfig().DbPort
	dbcollection := ReadConfig().DbCollection

	Mongoconnect()
	clientOpts := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%d", host, port))
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		fmt.Println("ERROR CONECTANDO A MONGO: ")
		log.Fatal(err)
		os.Exit(1)
	}

	for i := 0; i < len(allmicros); i++ {

		/// trim hash nombre micro y replicas

		regNombre, err := regexp.Compile("[^-a-zA-Z]+")
		if err != nil {
			log.Fatal(err)
		}

		regReplicas, err := regexp.Compile("[^0-9]+")
		if err != nil {
			log.Fatal(err)
		}

		replicaTrim := regReplicas.ReplaceAllString(allmicros[i], "")
		microTrim := regNombre.ReplaceAllString(allmicros[i], "")

		n, err := strconv.ParseInt(replicaTrim, 10, 0)

		final := CalculaPorcentaje(n)

		micros := Microservicios{
			Microservicio:         microTrim,
			Replicas:              replicaTrim,
			Porcentaje:            porcentaje,
			ReplicasConPorcentaje: final,
			Date:                  NowDate(),
			Hora:                  NowHora(),
			Usuario:               usuario,
			Namespace:             namespace,
		}

		finalString := strconv.FormatInt(final, 10)
		fmt.Println(finalString)
		cmd := exec.Command("bash", "-c", "libs/oc scale dc/"+microTrim+" --replicas="+finalString+" -n "+namespace)
		cmd.Wait()
		out, err := cmd.CombinedOutput()
		if err != nil {
			out2 := string([]byte(out))
			fmt.Println(out2)
			log.Fatalf("ERROR AL REALIZAR SCALE EN OPENSHIFT %s\n", err)
			fmt.Println("********************************************")
			os.Exit(1)
		}
		salida := string([]byte(out))
		fmt.Println("*********************************")
		fmt.Println("REALIZANDO SCALE...\n", salida)

		collection := client.Database(database).Collection(dbcollection)
		resultado, err := collection.InsertOne(context.TODO(), micros)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		resultado = nil
		fmt.Println("MICROSERVICIO REGISTRADO CORRECTAMENTE: \n Nombre: "+microTrim+"\n Replica: "+replicaTrim+"\n Porcentaje a reducir: ", porcentaje, "\n Replicas con porcentaje: ", final, "\n", resultado)
		fmt.Println("*********************************")
	}

	err = client.Disconnect(context.TODO())

	if err != nil {
		log.Fatal(err)
	}

}

func NowDate() string {

	current := time.Now()
	format := current.Format("2006-01-02")
	return format
}

func NowHora() string {

	current := time.Now()
	format := current.Format("15:04:05")
	return format
}

// openshift

func CheckOpenshiftNodos() {

	nodos := ReadConfig().OpenshiftNodos

	for i := 0; i < len(nodos); i++ {

		cmd := exec.Command("bash", "-c", "libs/oc get node "+nodos[i])
		cmd.Wait()
		out, err := cmd.CombinedOutput()
		if err != nil {
			out2 := string([]byte(out))
			fmt.Println(out2)
			log.Fatalf("FALLO AL OBTENER EL NODO EN OPENSHIFT %s\n", err)
			fmt.Println("********************************************")
			os.Exit(1)
		}

		fmt.Println("NODO VALIDO EN OPENSHIFT: " + nodos[i] + "\n")
		out3 := string([]byte(out))
		fmt.Println(out3)
		fmt.Println("********************************************")

	}

}

func CheckOpenshiftNamespace() {

	namespace := ReadConfig().OpenshiftNamespace

	cmd := exec.Command("bash", "-c", "libs/oc get project "+namespace)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil {
		out2 := string([]byte(out))
		fmt.Println(out2)
		log.Fatalf("FALLO AL OBTENER EL NAMESPACE EN OPENSHIFT %s\n", err)
		fmt.Println("********************************************")
		os.Exit(1)
	}

	fmt.Println("NAMESPACE VALIDO EN OPENSHIFT: " + namespace + "\n")
	out3 := string([]byte(out))
	fmt.Println(out3)
	fmt.Println("********************************************")

}

func CheckOpenshiftLogin() {

	user := ReadConfig().OpenshiftUser
	password := ReadConfig().OpenshiftPassword
	host := ReadConfig().OpenshiftHost

	cmd := exec.Command("bash", "-c", "libs/oc login "+host+" --username="+user+" --password="+password+" --insecure-skip-tls-verify=false")
	cmd.Wait()
	_, err := cmd.Output()
	if err != nil {
		fmt.Println("********************************************")
		log.Fatalf("FALLO CONEXION CON OPENSHIFT %s\n", err)
		fmt.Println("********************************************")
		os.Exit(1)
	}

	cmd2 := exec.Command("bash", "-c", "libs/oc whoami -t")
	cmd.Wait()
	out, err := cmd2.Output()
	if err != nil {
		fmt.Println("********************************************")
		log.Fatalf("FALLO CONEXION CON OPENSHIFT %s\n", err)
		fmt.Println("********************************************")
		os.Exit(1)
	}
	auth := string([]byte(out))
	fmt.Println("AUTENTICADO CORRECTAMENTE EN OPENSHIFT TOKEN GENERADO: ", auth)
	fmt.Println("********************************************")
}
