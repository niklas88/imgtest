/*
HornSchunk computes an optic flow field using the method of
Horn & Schunk and an iterative Jacobi scheme
*/
package main

import (
	"flag"
	"fmt"
	"github.com/harrydb/go/img/pnm"
	"github.com/niklas88/imgtest/floatimage"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
)

// Derivatives fx, fy, fz as 3 channel FloatImg to make access sane
const (
	Fxc = iota
	Fyc = iota
	Fzc = iota
)

func deriveMixed(f1, f2 *floatimage.FloatImg) *floatimage.FloatImg {
	const hx = 1.0
	const hy = 1.0
	bounds := f1.Bounds()
	derivs := floatimage.NewFloatImg(bounds, 3)
	for j := bounds.Min.Y + 1; j < bounds.Max.Y-1; j++ {
		for i := bounds.Min.X + 1; i < bounds.Max.X-1; i++ {
			Fx := (f1.At(i+1, j)[0] - f1.At(i-1, j)[0] + f2.At(i+1, j)[0] - f2.At(i-1, j)[0]) / (4.0 * hx)
			Fy := (f1.At(i, j+1)[0] - f1.At(i, j-1)[0] + f2.At(i, j+1)[0] - f2.At(i, j-1)[0]) / (4.0 * hy)
			Fz := f2.At(i, j)[0] - f1.At(i, j)[0]
			derivs.Set(i, j, Fxc, Fx)
			derivs.Set(i, j, Fyc, Fy)
			derivs.Set(i, j, Fzc, Fz)
		}
	}
	return derivs
}

func flow(alpha float64, derivs, vecField *floatimage.FloatImg) {
	bounds := vecField.Bounds()
	oldvec := floatimage.NewFloatImg(bounds, 2)
	// Copy old vector field
	for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
		for i := bounds.Min.X; i < bounds.Max.X; i++ {
			oldvec.Set(i, j, 0, vecField.At(i, j)[0])
			oldvec.Set(i, j, 1, vecField.At(i, j)[1])
		}
	}

	help := 1.0 / float32(alpha)
	var nn int
	var uSum, vSum float32

	for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
		for i := bounds.Min.X; i < bounds.Max.X; i++ {
			nn = 0
			uSum, vSum = 0, 0
			if i > bounds.Min.X {
				nn++
				uSum += oldvec.At(i-1, j)[0]
				vSum += oldvec.At(i-1, j)[1]
			}

			if i < bounds.Max.X-1 {
				nn++
				uSum += oldvec.At(i+1, j)[0]
				vSum += oldvec.At(i+1, j)[1]
			}

			if j > bounds.Min.Y {
				nn++
				uSum += oldvec.At(i, j-1)[0]
				vSum += oldvec.At(i, j-1)[1]
			}

			if j < bounds.Max.Y-1 {
				nn++
				uSum += oldvec.At(i, j+1)[0]
				vSum += oldvec.At(i, j+1)[1]
			}

			fxij := derivs.At(i, j)[Fxc]
			fyij := derivs.At(i, j)[Fyc]
			fzij := derivs.At(i, j)[Fzc]
			voldij := oldvec.At(i, j)[1]
			uoldij := oldvec.At(i, j)[0]
			uSum -= help * fxij * (fyij*voldij + fzij)
			uSum /= float32(nn) + help*fxij*fxij
			vSum -= help * fyij * (fxij*uoldij + fzij)
			vSum /= float32(nn) + help*fyij*fyij

			vecField.Set(i, j, 0, uSum)
			vecField.Set(i, j, 1, vSum)
		}
	}
}

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
			value := img.At(x, y)[0]
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
			temp := img.At(x, y)[0] - mean
			variance += temp * temp
		}
	}
	variance = float32(float64(variance) / imgsize)
	return
}

var finame1, finame2 string
var foutname string
var alpha float64
var iterations int

func init() {
	flag.StringVar(&finame1, "infile1", "img1.pgm", "The first image for optical flow computation")
	flag.StringVar(&finame2, "infile2", "img2.pgm", "The second image for optical flow computation")
	flag.StringVar(&foutname, "outfile", "out.pgm", "The flow magnitude image")
	flag.Float64Var(&alpha, "alpha", 1.4, "The smoothing weight alpha > 0")
	flag.IntVar(&iterations, "iterations", 100, "Number of iterations")
}

func main() {
	flag.Parse()
	fmt.Printf("Computing optical flow betwen %s and %s, result will be saved in %s\n", finame1, finame2, foutname)

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

	// Compute fx, fy, fz derivatives as FloatImg with 3 channels for faster access
	derivs := deriveMixed(f1, f2)

	bounds := img1.Bounds()
	// vector field as FloatImg with 2 channels
	vecField := floatimage.NewFloatImg(bounds, 2)
	// Magnitude image as FloatImg with 1 channel
	magImg := floatimage.NewFloatImg(bounds, 1)
	var min, max, mean, variance float32
	// Process image using the Jacobi method to incrementally compute the vector field
	for k := 1; k <= iterations; k++ {
		fmt.Printf("iteration number: %d \n", k)
		flow(alpha, derivs, vecField)

		/* calculate flow magnitude */
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			for i := bounds.Min.X; i < bounds.Max.X; i++ {
				tmp := vecField.At(i, j)[0]*vecField.At(i, j)[0] + vecField.At(i, j)[1]*vecField.At(i, j)[1]
				magImg.Set(i, j, 0, float32(math.Sqrt(float64(tmp))))
			}
		}
		min, max, mean, variance = analyse(magImg)
		fmt.Printf("min = %f, max = %f, mean = %f, variance = %f\n", min, max, mean, variance)
	}

	fout, err := os.Create(foutname)
	if err != nil {
		log.Fatal(err)
	}
	defer fout.Close()
	mag := floatimage.GrayFloatNoDummiesToImage(magImg)

	err = pnm.Encode(fout, mag, pnm.PGM)
	if err != nil {
		log.Fatal(err)
	}

}
