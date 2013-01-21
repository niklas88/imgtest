/*
HornSchunk computes an optic flow field using the method of
Horn & Schunk and an iterative Jacobi scheme
*/
package main

import (
	"flag"
	"fmt"
	"github.com/harrydb/go/img/pnm"
	"github.com/niklas88/imgtest/algorithms"
	"github.com/niklas88/imgtest/floatimage"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
)

func analyse(img *floatimage.FloatImg) (min, max, mean, variance float32) {
	var sum float64
	bounds := img.Bounds()
	if img.Bounds().Empty() {
		return 0, 0, 0, 0
	}

	sum = 0.0
	min = img.Pix[0]
	max = img.Pix[0]
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
			value := img.AtF(x, y)[0]
			if value < min {
				min = value
			}
			if value > max {
				max = value
			}
			sum += float64(value)
		}
	}
	imgsize := float64(bounds.Dx()-2) * float64(bounds.Dy()-2)
	mean = float32(sum / imgsize)
	variance = 0.0
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
			temp := img.AtF(x, y)[0] - mean
			variance += temp * temp
		}
	}
	variance = float32(float64(variance) / imgsize)
	return
}

var finame1, finame2 string
var magImageName, dirImageName string
var alpha float64
var iterations int

func init() {
	flag.StringVar(&finame1, "infile1", "img1.pgm", "The first image for optical flow computation")
	flag.StringVar(&finame2, "infile2", "img2.pgm", "The second image for optical flow computation")
	flag.StringVar(&magImageName, "magimg", "mag.pgm", "The flow magnitude image")
	flag.StringVar(&dirImageName, "dirimg", "direction.ppm", "The flow direction image")
	flag.Float64Var(&alpha, "alpha", 100.0, "The smoothing weight alpha > 0")
	flag.IntVar(&iterations, "iterations", 160, "Number of iterations")
}

func main() {
	flag.Parse()
	fmt.Printf("Computing optical flow betwen %s and %s, result will be saved in %s and %s\n", finame1, finame2, magImageName, dirImageName)

	fin1, err := os.Open(finame1)
	if err != nil {
		log.Fatal(err)
	}
	defer fin1.Close()

	img1, _, err := image.Decode(fin1)
	if err != nil {
		log.Fatal(err)
	}

	fin2, err := os.Open(finame2)
	if err != nil {
		log.Fatal(err)
	}
	defer fin2.Close()

	img2, _, err := image.Decode(fin2)
	if err != nil {
		log.Fatal(err)
	}

	if !img1.Bounds().Eq(img2.Bounds()) {
		log.Fatal("The image bounds need to match")
	}

	// Create Gray float based images with overlap for mirroring boundaries
	f1 := floatimage.GrayFloatWithDummiesFromImage(img1)
	f2 := floatimage.GrayFloatWithDummiesFromImage(img2)

	min1, max1, mean1, var1 := analyse(f1)
	min2, max2, mean2, var2 := analyse(f2)
	fmt.Printf("min1 = %f, max1 = %f, mean1 = %f, var1 = %f\n", min1, max1, mean1, var1)
	fmt.Printf("min2 = %f, max2 = %f, mean2 = %f, var2 = %f\n", min2, max2, mean2, var2)

	uv := algorithms.OpticFlowHornSchunk(f1, f2, float32(alpha), iterations)
	magImg := algorithms.MagImage(uv)

	fout, err := os.Create(magImageName)
	if err != nil {
		log.Fatal(err)
	}
	defer fout.Close()
	magImg.ScaleToUnsignedByte()
	mag := magImg.Dedummify()

	err = pnm.Encode(fout, mag, pnm.PGM)
	if err != nil {
		log.Fatal(err)
	}

	uv.ColorFunc = func(x, y int, d []float32) color.Color {
		const mult = 100.0
		g := magImg.AtF(x, y)[0]
		return color.YCbCr{floatimage.Tu8c(g), floatimage.Tu8c(d[0]*mult + 127.5), floatimage.Tu8c(d[1]*mult + 127.5)}
	}

	uv.ColorModelFunc = func() color.Model {
		return color.YCbCrModel
	}

	fout2, err := os.Create(dirImageName)
	if err != nil {
		log.Fatal(err)
	}
	defer fout2.Close()

	err = pnm.Encode(fout2, uv.Dedummify(), pnm.PPM)
	if err != nil {
		log.Fatal(err)
	}
}
