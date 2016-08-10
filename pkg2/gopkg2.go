package temp

import "github.com/vadmeste/temp/pkg1"

func unexportedFunctionPkg2() {
	unexportedFunctionPkg1()
}
