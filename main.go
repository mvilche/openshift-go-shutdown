package main

import (
	"fmt"
)

func main() {

	fmt.Println("********************************************")
	fmt.Println("Proyecto: Apagado automatizado de nodos en openshift")
	fmt.Println("Autor: Martin Fabrizzio Vilche")
	fmt.Println("Version: ", NowDate(), NowHora())
	fmt.Println("********************************************\n")

	fmt.Println("Iniciando comprobacion de los datos ingresados..\n")
	ReadConfig()
	Mongoconnect()
	CheckOpenshiftLogin()
	CheckOpenshiftNodos()
	CheckOpenshiftNamespace()
	OpenshiftGetReplicas()

}
