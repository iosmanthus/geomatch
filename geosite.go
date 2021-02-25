package geomatch

import (
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	v2ray "v2ray.com/core/app/router"
)

var _ Matcher = (*DomainMatcher)(nil)
var _ Matcher = (*FullMatcher)(nil)
var _ Matcher = (*SubDomainMatcher)(nil)
var _ Matcher = (*RegexMatcher)(nil)
var _ Matcher = (*KeywordMatcher)(nil)

type DomainMatcher struct {
	matchers       []Matcher
	matcherCondIdx []int
	conditions     []Condition
}

type Matcher interface {
	Match(domain string) []Condition
}

func (m *DomainMatcher) Match(domain string) []Condition {
	l := len(domain)
	if l > 0 && domain[l-1] == '.' {
		domain = domain[:l-1]
	}
	var conditions []Condition
	for i, matcher := range m.matchers {
		if cond := matcher.Match(domain); cond != nil {
			condition := m.conditions[m.matcherCondIdx[i]]
			if condition.prefix == "geosite" {
				condition.addChildren(cond)
			}
			conditions = append(conditions, condition)
		}
	}
	return conditions
}

type KeywordMatcher string

func NewKeywordMatcher(keyword string) *KeywordMatcher {
	m := KeywordMatcher(keyword)
	return &m
}

func (m *KeywordMatcher) Match(domain string) []Condition {
	var condition []Condition
	keyword := string(*m)
	if strings.Contains(domain, keyword) {
		condition = append(condition, newCondition("keyword", keyword))
	}
	return condition
}

type FullMatcher string

func NewFullMatcher(content string) *FullMatcher {
	m := FullMatcher(content)
	return &m
}

func (m *FullMatcher) Match(domain string) []Condition {
	var condition []Condition
	s := string(*m)
	if s == domain {
		condition = append(condition, newCondition("full", s))
	}
	return condition
}

type RegexMatcher struct {
	*regexp.Regexp
}

func NewRegexMatcher(pattern string) (*RegexMatcher, error) {
	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexMatcher{r}, nil
}

func (m *RegexMatcher) Match(domain string) []Condition {
	var condition []Condition
	if m.Regexp.MatchString(domain) {
		condition = append(condition, newCondition("regexp", m.Regexp.String()))
	}
	return condition
}

type SubDomainMatcher string

func NewSubDomainMatcher(subdomain string) *SubDomainMatcher {
	m := SubDomainMatcher(subdomain)
	return &m
}

func (m *SubDomainMatcher) Match(domain string) []Condition {
	var condition []Condition

	subdomain := string(*m)
	diff := len(domain) - len(subdomain)

	if diff < 0 {
		return condition
	}
	if domain[diff:] == subdomain && (domain[:diff] == "" || domain[diff-1] == '.') {
		condition = append(condition, newCondition("domain", subdomain))
	}
	return condition
}

type DomainMatcherBuilder struct {
	file       string
	conditions []string
}

type Condition struct {
	prefix   string
	payload  string
	children []Condition
}

func newCondition(prefix, payload string) Condition {
	return Condition{prefix, payload, nil}
}

func (c *Condition) addChildren(conditions []Condition) {
	c.children = append(c.children, conditions...)
}

func (c Condition) String() string {
	s := c.prefix + ":" + c.payload
	for _, cond := range c.children {
		s += "/" + cond.String()
	}
	return s
}

func NewDomainMatcherBuilder() *DomainMatcherBuilder {
	return &DomainMatcherBuilder{}
}

func (b *DomainMatcherBuilder) From(file string) *DomainMatcherBuilder {
	b.file = file
	return b
}

func (b *DomainMatcherBuilder) AddConditions(conditions ...string) *DomainMatcherBuilder {
	b.conditions = append(b.conditions, conditions...)
	return b
}

func (b *DomainMatcherBuilder) buildConditions() ([]Condition, error) {
	validConditionPrefix := []string{"keyword", "regexp", "full", "domain", "geosite"}

	var conditions []Condition
	for _, cond := range b.conditions {
		valid := false
		for _, prefix := range validConditionPrefix {
			if valid = strings.HasPrefix(cond, prefix+":"); valid {
				conditions = append(conditions,
					newCondition(prefix, cond[len(prefix)+1:]))
				break
			}
		}

		if !valid {
			return nil, errors.Errorf("invalid Condition format: %s", cond)
		}
	}

	return conditions, nil
}

func extractAttr(payload string) (string, string, error) {
	idx := strings.Index(payload, "@")
	if idx == len(payload)-1 {
		return "", "", errors.Errorf("invalid geosite content: geosite:%s", payload)
	}
	if idx == -1 {
		return payload, "", nil
	}
	return payload[:idx], payload[idx+1:], nil
}

func extractDomainList(payload string, geoSiteList *v2ray.GeoSiteList) ([]*v2ray.Domain, error) {
	countryCode, expectedAttr, err := extractAttr(payload)
	if err != nil {
		return nil, err
	}

	countryCode = strings.ToUpper(countryCode)
	for _, entry := range geoSiteList.GetEntry() {
		if countryCode == entry.GetCountryCode() {
			if expectedAttr != "" {
				var domains []*v2ray.Domain
				for _, domain := range entry.GetDomain() {
					matched := false
					for _, attr := range domain.GetAttribute() {
						if expectedAttr == attr.GetKey() {
							matched = true
							break
						}
					}
					if matched {
						domains = append(domains, domain)
					}
				}
				return domains, nil
			}
			return entry.GetDomain(), nil
		}
	}

	return nil, errors.Errorf("domain list for geosite:%s not found", payload)
}

func (b *DomainMatcherBuilder) Build() (*DomainMatcher, error) {
	conditions, err := b.buildConditions()
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(b.file)
	if err != nil {
		return nil, err
	}

	geoList := new(v2ray.GeoSiteList)
	if err = proto.Unmarshal(data, geoList); err != nil {
		return nil, err
	}

	mapToV2RayDomainType := map[string]v2ray.Domain_Type{
		"keyword": v2ray.Domain_Plain,
		"regexp":  v2ray.Domain_Regex,
		"full":    v2ray.Domain_Full,
		"domain":  v2ray.Domain_Domain,
	}

	var (
		matchers       []Matcher
		matcherCondIdx []int
	)
	// Extract all the domains according to the conditions
	for i, cond := range conditions {
		var domains []*v2ray.Domain
		switch cond.prefix {
		case "geosite":
			extracted, err := extractDomainList(cond.payload, geoList)
			if err != nil {
				return nil, err
			}
			domains = append(domains, extracted...)
		default:
			domains = append(domains, &v2ray.Domain{
				Type:  mapToV2RayDomainType[cond.prefix],
				Value: cond.payload,
			})
		}

		// Construct matchers for DomainMatcher
		for _, domain := range domains {
			// Attach matcher's Condition index
			matcherCondIdx = append(matcherCondIdx, i)
			switch domain.Type {
			case v2ray.Domain_Plain:
				matchers = append(matchers, NewKeywordMatcher(domain.Value))
			case v2ray.Domain_Full:
				matchers = append(matchers, NewFullMatcher(domain.Value))
			case v2ray.Domain_Domain:
				matchers = append(matchers, NewSubDomainMatcher(domain.Value))
			case v2ray.Domain_Regex:
				matcher, err := NewRegexMatcher(domain.Value)
				if err != nil {
					return nil, err
				}
				matchers = append(matchers, matcher)
			}
		}
	}

	return &DomainMatcher{matchers: matchers, matcherCondIdx: matcherCondIdx, conditions: conditions}, nil
}
