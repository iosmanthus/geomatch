package geomatch

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const geoSiteDatPath = "./etc/geosite.dat"

func stringify(x interface{}) string {
	return fmt.Sprintf("%s", x)
}

func TestDomainMatch(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("geosite:google").
		AddConditions("geosite:microsoft").
		Build()
	assert.Nil(t, err)
	assert.NotNil(t, matcher.Match("www.google.com"))
	assert.NotNil(t, matcher.Match("www.microsoft.com"))

	builder = NewDomainMatcherBuilder()
	matcher, err = builder.From(geoSiteDatPath).
		AddConditions("geosite:geolocation-!cn").
		Build()
	assert.Nil(t, err)
	assert.NotNil(t, matcher.Match("www.google.com"))
	assert.Nil(t, matcher.Match("www.baidu.com"))
	assert.Nil(t, matcher.Match("www.bilibili.com"))
}

func TestFullMatch(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("full:www.baidu.com").
		AddConditions("full:www.google.com").
		Build()
	assert.Nil(t, err)

	assert.NotNil(t, matcher.Match("www.google.com"))
	assert.NotNil(t, matcher.Match("www.baidu.com"))

	assert.Nil(t, matcher.Match("www.example.com"))
	assert.Nil(t, matcher.Match("baidu.com"))
	assert.Nil(t, matcher.Match("google.com"))
}

func TestRegexMatch(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("regexp:.*baidu.com").
		AddConditions("regexp:.*google.com").
		Build()
	assert.Nil(t, err)

	result := matcher.Match("google.com")
	assert.NotNil(t, result)
	assert.Equal(t, "[regexp:.*google.com]", stringify(result))

	assert.NotNil(t, matcher.Match("baidu.com"))
	assert.NotNil(t, matcher.Match("xbaidu.com"))
	assert.NotNil(t, matcher.Match("xgoogle.com"))

	assert.Nil(t, matcher.Match("www.example.com"))

	builder = NewDomainMatcherBuilder()
	_, err = builder.From(geoSiteDatPath).
		AddConditions("regexp:*baidu.com").
		Build()
	assert.NotNil(t, err)
}

func TestSubDomainMatch(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("domain:baidu.com").
		AddConditions("domain:google.com").
		Build()
	assert.Nil(t, err)

	result := matcher.Match("dig.google.com")
	assert.Equal(t, "[domain:google.com]", stringify(result))

	assert.Nil(t, matcher.Match("ibaidu.com"))

	assert.NotNil(t, matcher.Match("www.google.com"))
	assert.NotNil(t, matcher.Match("www.baidu.com"))
	assert.NotNil(t, matcher.Match("google.com"))
	assert.NotNil(t, matcher.Match("baidu.com"))
	assert.Nil(t, matcher.Match("xbaidu.com"))
	assert.Nil(t, matcher.Match("igoogle.com"))
}

func TestKeywordMatch(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("keyword:video").
		AddConditions("keyword:baidu").
		Build()
	assert.Nil(t, err)

	result := matcher.Match("www.pornvideo.com")
	assert.NotNil(t, result)
	assert.Equal(t, "[keyword:video]", stringify(result))

	assert.NotNil(t, matcher.Match("www.googlevideo.com"))
	assert.NotNil(t, matcher.Match("www.baidu.com"))
	assert.NotNil(t, matcher.Match("baidu.com"))
	assert.NotNil(t, matcher.Match("xbaidux.com"))

	assert.Nil(t, matcher.Match("google.com"))
	assert.Nil(t, matcher.Match("www.google.com"))
	assert.Nil(t, matcher.Match("igoogle.com"))
}

func TestAttributes(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("geosite:google@ads").
		Build()
	assert.Nil(t, err)

	assert.NotNil(t, matcher.Match("googleoptimize.com"))
	assert.Nil(t, matcher.Match("www.google.com"))
}

func Test(t *testing.T) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("geosite:geolocation-!cn").
		AddConditions("geosite:microsoft").
		AddConditions("geosite:mozilla").
		AddConditions("geosite:gfw").
		Build()
	assert.Nil(t, err)
	assert.Equal(
		t,
		"[geosite:geolocation-!cn/domain:microsoft.com geosite:microsoft/domain:microsoft.com geosite:microsoft/full:www.microsoft.com]",
		stringify(matcher.Match("www.microsoft.com")),
	)
	assert.Equal(
		t,
		"[geosite:geolocation-!cn/domain:youtube.com geosite:gfw/domain:youtube.com]",
		stringify(matcher.Match("www.youtube.com")),
	)
}

func BenchmarkDomainMatcher_Match(b *testing.B) {
	builder := NewDomainMatcherBuilder()
	matcher, err := builder.From(geoSiteDatPath).
		AddConditions("geosite:geolocation-!cn").
		Build()
	assert.Nil(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match("www.google.com")
	}
}
