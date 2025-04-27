package sensitive_words

import (
	"fmt"
	"strings"
	"testing"
)

func TestCap(t *testing.T) {
	words := strings.Split(invalidWords,",")
	for _, v := range words {
		invalidWord[v] = nil
	}
	//Set := make(map[string]interface{}, 0)
	set["你妈逼的"] = nil
	set["你妈"] = nil
	set["狗日"] = nil
	addSensitiveToMap(set)
	text := "文明用语你&* 妈, 逼的你这个狗 日的，怎么这么傻啊。我也是服了，我日,这些话我都说不出口"
	fmt.Println(ChangeSensitiveWords(text))
	text = "no sensitive"
	fmt.Println(ChangeSensitiveWords(text))
}


