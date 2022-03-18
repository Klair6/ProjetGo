package main

import (
	"bufio"       //entrée sortie
	"fmt"         //print
	"image"       //image
	"image/color" //couleur des pixels
	"log"         //trace
	"net"         //socket
	"os"
	"strconv" //conversion avec des string
	"strings"

	"gocv.io/x/gocv" //librairie gocv
)

//variables globales et constantes

var touche string = ""

const PORT = "27001" //port choisi aléatoirement
const BUFFERSIZE = 1024

func camera(no_device int, connection net.Conn) {
	//fmt.Println("start device ", no_device)
	var newmat gocv.Mat //declaration ici car pb de compilation si déclarée dans un if

	webcam, err := gocv.VideoCaptureDevice(no_device) //premier acces a la camera
	if err != nil {
		fmt.Println("Ne peut pas initialiser la camera : ", err)
		return
	}
	defer webcam.Close() //ferme quand plus utilisé

	img := gocv.NewMat() // creer une matrice d'image
	defer img.Close()

	title := "Floutage visages camera n° :" + strconv.Itoa(no_device) //on conv no_device (int) en str pour faire +
	window := gocv.NewWindow(title)                                   //creer la fenetre graphique avec titre
	defer window.Close()

	// charger le classifieur pour reconnaitre qqch à partir de gocv
	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()

	//charger un modele de reconnaissance (ici visage frontal)
	if !classifier.Load("C:\\opencv\\haar-cascade-files-master\\haarcascade_frontalface_default.xml") {
		log.Fatal("Erreur chargement du fichier: data/haarcascade_frontalface_default.xml")
	}

	fmt.Println("Demarrage lecture camera n°: ", no_device)

	for { //boucle infinie pour lire et traiter chaque image de la camera
		if ok := webcam.Read(&img); !ok { //lire une image de la camera et affecte cette image dans la matrice img (référencée par son adresse)
			log.Fatal("ne peut pas lire camera n° :", no_device)
		}

		if touche == "c\r\n" { //on active l'option floutage de la vidéo uniquement avec la touche 'c'
			newmat = DetectionVisageFloutage(img, classifier) //fonction qui detecte les visages, convertit l'image, la floute , la reconvertit
		} else {
			newmat = img //image non floutée
		}

		if no_device == 0 && touche == "s\r\n" { //screenshot uniquement sur device 0
			touche = "" //on reinitialise la valeur de touche pour ne faire qu'une fois le screenshot (et non pas toutes les 100ms)
			screenshotclient(img, connection)
		}

		// afficher la fenetre contenant la matrice et attendre 100 ms
		window.IMShow(newmat)
		window.WaitKey(100)
	}
}

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
// on divise le rectangle en carrés de 64 x 64 bits
// on calcule la moyenne de chaque couleurs d'un carré et on affecte cette couleur a tout le carré
func blurMaison(imageInOut *image.RGBA, rectangle image.Rectangle) { //on retourne la meme image qu'en entrée mais modifiée

	TAILLE_CARRE := 64 //on fait des carrés de ce nombre de pixels
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
func NewMatRGB8FromImage(img image.Image) (gocv.Mat, error) { //return renvoie 2 parametres matrice ou erreur
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
func screenshotclient(img gocv.Mat, connection net.Conn) {

	img_jpg, _ := gocv.IMEncode(".jpg", img) //gocv.Mat to *gocvNativeByteBuffer en utilisant le format jpg

	//img_bytes := img.ToBytes() //on conv img (gocv.Mat) en bytes pour l'envoyer dans la socket

	EnvoiImage(img_jpg.GetBytes(), connection) //getBytes = from *gocvNativeByteBuffer to bytes

	img_blured_bytes := ReceptionImage(connection)

	img_screenshot, _ := gocv.IMDecode(img_blured_bytes, 1) //on decode des bytes au format jpg (1) pr avoir une gocv.Mat

	title_screenshot := "Screenshot on camera n° 0 "
	window_screenshot := gocv.NewWindow(title_screenshot)

	//defer window_screenshot.Close() mis en commentaire car sinon on sort de la fonction screenshot et l'image ne reste pas

	// afficher la fenetre contenant le screenshot et attendre 100 ms
	fmt.Println("affichage screen")
	window_screenshot.IMShow(img_screenshot)
	window_screenshot.WaitKey(100)

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
	fmt.Println("Debut envoie image")

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

	fmt.Println("En attente de reception de l'image floutée ")

	bufferImageSize := make([]byte, 10) //creation du buffer de taille 10 bytes visant a contenir la taille de l'image

	connection.Read(bufferImageSize)                         //on recupere les 10 premiers octets contenant la taille de l'image
	cut_buffer := strings.Trim(string(bufferImageSize), ":") //on enleve les ":" a la chaine pour avoir uniquement la taille de l'image
	imageSize, _ := strconv.ParseInt(cut_buffer, 10, 64)     //conversion from string to int64 en base 10

	//fmt.Println("Image size:", imageSize, " bufsize:", bufferImageSize, "   ", string(bufferImageSize))

	if imageSize == 0 { //pb dans la transmission de la taille
		fmt.Println("Image size is zero")
		return buffImage //retourne un buffer vide
	}

	var receivedBytes int64 = 0

	for { //on lit les paquets de 1024 bytes pour reconstituer l'image //on boucle sur chaque paquets
		if (imageSize - receivedBytes) <= BUFFERSIZE { //cas du dernier paquet

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

	fmt.Println("Début programme Client")

	serveurip := "localhost:" + PORT

	connection, err := net.Dial("tcp", serveurip) // fonction qui ouvre la connexion entre le serveur et le client en local sur un port défini
	if err != nil {
		fmt.Println("connexion au serveur impossible, execution en mode partiel")
	} else {
		fmt.Println("Connecté au serveur!")
		defer connection.Close()
	}
	go camera(0, connection)
	go camera(1, connection)

	fmt.Println("Appuyer sur 'q' pour sortir, 'c' pour flouter, 's' pour envoyer l'image en cours au serveur et la récuperer floutée")
	fmt.Println("Appuyer sur toute autre touche pour revenir au mode initial")

	for { //boucle infinie pour laisser les go routines camera s'executer de leur coté
		reader := bufio.NewReader(os.Stdin)  //on creer le reader sur l'entrée clavier
		choice, _ := reader.ReadString('\n') //on lit jusqu'au \n
		touche = string(choice)

		if touche == "q\r\n" { //on quitte le programme
			break
		}
	}
	fmt.Println("Fin programme Client")

}

/*
1ere exécution on doit faire :
- aller dans mon repertoire : C:\Claire F\INSA LYON\COURS\ELP\Goprojet\camera
- go mod init nom_prgm.go
- go mod tidy => crée un go.sum (certificat?) + go.mod (nom module + les librairies necessaires)
- go build => compile le programme et genere le .exe
- go run nom_prgm
*/
