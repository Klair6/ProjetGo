package main

import (
	"fmt"         //print
	"image"       //image
	"image/color" //couleur des pixels
	"log"         //trace
	"net"         //socket
	"strconv"     //conversion avec des string
	"strings"

	"gocv.io/x/gocv" //librairie gocv
)

//variables globales et constantes

const PORT = "27001" //port choisi aléatoirement
const BUFFERSIZE = 1024

//fonction qui detecte les visages, convertit l'image, la floute , la reconvertit
func DetectionVisageFloutage(img gocv.Mat, classifier gocv.CascadeClassifier) gocv.Mat {
	Img_modifiable, err := img.ToImage() // image.ToImage est la fonction qui convertie une matrice gocv.Mat en une image.image (modifiable)
	if err != nil {
		log.Fatal("erreur conversion matricegocv en image.image ", err)
	}

	Img_RGBA, ok := Img_modifiable.(*image.RGBA) //On passe finalImg en image de type RGBA qui est un sous type de image.image
	if !ok {
		log.Fatal("Image pas de type rgba, et donc non modifiable")
	}

	// detection visages qui sont retournés dans une liste de rectangles
	rects := classifier.DetectMultiScale(img)

	// pour chaque rectangle (visage)
	for _, rect := range rects { // _ recupere l'indice dont on n'a pas besoin et rect recupere l'elmt de la liste
		go blurMaison(Img_RGBA, rect) //on crée une goroutine sans attendre sa fin pour flouter l'interieur d'un rectangle dans Img_RGBA
		//gocv.GaussianBlur(imgFace, &imgFace, image.Pt(75, 75), 0, 0, gocv.BorderDefault) on aurait pu utiliser cette fonction de flouttage
	}

	newmat, err := NewMatRGB8FromImage(Img_RGBA) //fonction qui convertit image RGBA en matrice gocv
	if err != nil {
		log.Fatal(err)
	}
	return newmat

}

// flouter l'interieur d'un rectangle dans Img_RGBA
// on divise le rectangle en carrés de 16 x 16 bits
// on calcule la moyenne de chaque couleurs d'un carré et on affecte cette couleur a tout le carré
func blurMaison(imageInOut *image.RGBA, rectangle image.Rectangle) { //on retourne la meme image qu'en entrée mais modifiée

	TAILLE_CARRE := 16 //on fait des carrés de ce nombre de pixels
	SURFACE_CARRE := uint32(TAILLE_CARRE * TAILLE_CARRE)

	bounds := rectangle.Bounds() //on recupere les contours du rectangle
	min := bounds.Min            // point min, en haut a gauche
	max := bounds.Max            // point max, en bas a droite

	for y := min.Y; y < max.Y; y += TAILLE_CARRE { //on boucle sur chaque carré en y et en x
		for x := min.X; x < max.X; x += TAILLE_CARRE {
			//a chaque nouveau carré initialisation des totaux rgba en 32bits
			rtot := uint32(0)
			gtot := uint32(0)
			btot := uint32(0)
			atot := uint32(0)

			for yy := 0; yy < TAILLE_CARRE; yy++ { //on boucle sur l'interieur de chaque carré (on commence en haut a gauche)
				for xx := 0; xx < TAILLE_CARRE; xx++ {
					r, g, b, a := imageInOut.At(x+xx, y+yy).RGBA() //on recupere les valeurs rgba actuelles d'un pixel de l'image en 32 bits
					rtot += r
					gtot += g
					btot += b
					atot += a
				}
			}
			rmoy32 := rtot / SURFACE_CARRE // moyenne rouge
			gmoy32 := gtot / SURFACE_CARRE // moyenne vert
			bmoy32 := btot / SURFACE_CARRE // moyenne bleu
			amoy32 := atot / SURFACE_CARRE // moyenne opacité

			//conversion de 32 bits à 8 bits
			rmoy8 := uint8(rmoy32 >> 8)
			gmoy8 := uint8(gmoy32 >> 8)
			bmoy8 := uint8(bmoy32 >> 8)
			amoy8 := uint8(amoy32 >> 8)

			for yy := 0; yy < TAILLE_CARRE; yy++ { //on boucle sur l'interieur de chaque carré (on commence en haut a gauche)
				for xx := 0; xx < TAILLE_CARRE; xx++ {
					imageInOut.Set(x+xx, y+yy, color.RGBA{rmoy8, gmoy8, bmoy8, amoy8}) //on affecte les memes valeurs rgba a tous les pixels du carré
				}
			}
		}
	}
}

// fonction qui convertit image RGBA en matrice gocv
func NewMatRGB8FromImage(img image.Image) (gocv.Mat, error) { //return renvoie 2 parametres matrice et erreur
	bounds := img.Bounds()               //recupere les contours de l'image
	x := bounds.Dx()                     //recupere la largeur
	y := bounds.Dy()                     //recupere la hauteur
	list_bytes := make([]byte, 0, x*y*4) // on cree une liste bytes de dimension x 4 (r,g,b,a)
	// on aurait pu mettre 3 sans le a, apres mettre MatTypeCV8UC3 et mettre _ au lieu de a

	// colonne avant ligne car sinon image pas correcte quand on remplit la liste bytes
	for j := bounds.Min.Y; j < bounds.Max.Y; j++ { //on boucle sur les colonnes de l'ordonnée de début à l'ordonnée de fin
		for i := bounds.Min.X; i < bounds.Max.X; i++ { //on boucle sur les lignes de l'abscisse de début à l'abscisse de fin
			r, g, b, a := img.At(i, j).RGBA()                                               // pour chaque point, on récupere les valeurs RGBA par defaut en 32 bits (0 à 65 535)
			list_bytes = append(list_bytes, byte(b>>8), byte(g>>8), byte(r>>8), byte(a>>8)) //on remplit la liste bytes en convertissant les valeurs en 8 bits (0 à 255)
		}
	}
	return gocv.NewMatFromBytes(y, x, gocv.MatTypeCV8UC4, list_bytes) //convertit la liste bytes en matrice gocv en utlisant le modele RBGA 8 bits
}

//traitement screenshot
func screenshotserveur(connection net.Conn, classifier gocv.CascadeClassifier) {

	for { //permet de recevoir plusieurs screenshot
		img_bytes := ReceptionImage(connection)

		img_screenshot, _ := gocv.IMDecode(img_bytes, 1) //on decode des bytes au format jpg (1) pr avoir une gocv.Mat

		img_blured_mat := DetectionVisageFloutage(img_screenshot, classifier)

		img_blured_NBB, _ := gocv.IMEncode(".jpg", img_blured_mat) //gocv.Mat to *gocvNativeByteBuffer en utilisant le format jpg

		EnvoiImage(img_blured_NBB.GetBytes(), connection) //getBytes = from *gocvNativeByteBuffer to bytes
	}

}

func EnvoiImage(img_bytes []byte, connection net.Conn) {

	//envoie de la taille de l'image au serveur
	//fillString = comble une chaine a 10 caracteres (Le second 10)
	//FormatInt = transforme un int en string en utlisant la base 10
	Img_size := fillString(strconv.FormatInt(int64(len(img_bytes)), 10), 10) //bufSize = string
	fmt.Println("taille image =", len(img_bytes), " buff=", Img_size)
	connection.Write([]byte(Img_size)) //.write = envoie de bytes dans un socket //[]byte = cast string en bytes

	var sentBytes int64
	sentBytes = 0
	fmt.Println("Start sending image")

	for { //on divise l'image en paquets de bytes de lataille du buffersize (1024 bytes)
		if (int64(len(img_bytes)) - sentBytes) <= BUFFERSIZE { //dernier envoie de paquet

			sendBuffer := img_bytes[sentBytes:int64(len(img_bytes))] //sendBuffer = dernier paquet de l'image bytes
			connection.Write(sendBuffer)
			//fmt.Println("Image envoyé de l'index", sentBytes, " à:", int64(len(img_bytes)))

			break
		}

		//cas classique de paquet de 1024 bytes
		sendBuffer := img_bytes[sentBytes : sentBytes+BUFFERSIZE]
		connection.Write(sendBuffer)
		//fmt.Println("keep sending image:", noimg, "from index:", sentBytes, " to:", sentBytes+BUFFERSIZE)

		sentBytes += BUFFERSIZE
	}
	fmt.Println("Fin envoi de l'image")
}

func ReceptionImage(connection net.Conn) []byte {

	var buffImage []byte //buffer qui va contenir l'image complete

	fmt.Println("En attente de reception de l'image a flouter ")

	bufferImageSize := make([]byte, 10) //creation du buffer de taille 10 bytes contenant la taille de l'image

	connection.Read(bufferImageSize)                         //lit le msg contenant la taille de l'image
	cut_buffer := strings.Trim(string(bufferImageSize), ":") //on enleve les ":" a la chaine pour avoir uniquement la taille de l'image
	imageSize, _ := strconv.ParseInt(cut_buffer, 10, 64)     //conversion from string to int64 en base 10

	//fmt.Println("Image size:", imageSize, " bufsize:", bufferImageSize, "   ", string(bufferImageSize))

	if imageSize == 0 { //pb dans la transmission de la taille
		fmt.Println("Image size is zero")
		return buffImage //retourne un buffer vide
	}

	var receivedBytes int64 = 0

	for { //on lit les paquets de 1024 bytes pour reconstituer l'image //on boucle sur chaque paquets
		if (imageSize - receivedBytes) < BUFFERSIZE { //cas du dernier paquet

			buffPartImage := make([]byte, imageSize-receivedBytes) //creation d'un buffer uniquement pour le dernier paquet
			connection.Read(buffPartImage)                         //on lit le dernier paquet
			buffImage = append(buffImage, buffPartImage...)        //on rajoute le dernier paquet dans le buffimage
			break
		}
		buffPartImage := make([]byte, BUFFERSIZE)
		connection.Read(buffPartImage)
		buffImage = append(buffImage, buffPartImage...) //on rajoute un paquet dans le buffimage
		//fmt.Println("Keep receiving image:", noimg, " index:", receivedBytes)
		receivedBytes += BUFFERSIZE //on passe au prochain paquet
	}
	fmt.Println("On a recu l'image complete de taille :", imageSize)

	return buffImage //en byte
}

func fillString(returnString string, toLength int) string { //comble le msg de 10 octet
	for {
		lengtString := len(returnString)
		if lengtString < toLength {
			returnString = returnString + ":"
			continue
		}
		break
	}
	return returnString
}

func main() {

	fmt.Println("Début programme Serveur")

	serveurip := "localhost:" + PORT

	// charger le classifieur pour reconnaitre qqch à partir de gocv
	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()

	//charger un modele de reconnaissance (ici visage frontal)
	if !classifier.Load("C:\\opencv\\haar-cascade-files-master\\haarcascade_frontalface_default.xml") {
		log.Fatal("Erreur chargement du fichier: data/haarcascade_frontalface_default.xml")
	}

	serveur, err := net.Listen("tcp", serveurip) //serveur en attente sur la socket d'écoute
	if err != nil {
		log.Fatal("Erreur sur la socket d'écoute: ", err)
	}
	defer serveur.Close()
	fmt.Println("Serveur en attente de connections...")

	for { //boucle infinie sur attente de connection
		connection, err := serveur.Accept() //il y a une connection et on attribue un id unique (connection)
		if err != nil {
			log.Fatal("Erreur sur la socket d'écoute: ", err)
		}

		fmt.Println("Client connecté")

		go screenshotserveur(connection, classifier) //go routine au cas ou il y a plusieurs clients
	}

	fmt.Println("Fin programme serveur")

}

/*
1ere exécution on doit faire :
- aller dans mon repertoire : C:\Claire F\INSA LYON\COURS\ELP\Goprojet\camera
- go mod init nom_prgm.go
- go mod tidy => crée un go.sum (certificat?) + go.mod (nom module + les librairies necessaires)
- go build => compile le programme et genere le .exe
- go run nom_prgm
*/
