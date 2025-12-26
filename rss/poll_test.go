package rss

import (
	"encoding/xml"
	"testing"
)

const (
	item1 string = `        <item>
            <title><![CDATA[@opencode-ai/plugin]]></title>
            <link>https://npmjs.com/package/@opencode-ai/plugin</link>
            <guid isPermaLink="true">https://npmjs.com/package/@opencode-ai/plugin</guid>
            <dc:creator><![CDATA[GitHub Actions]]></dc:creator>
            <pubDate>Sun, 21 Dec 2025 03:07:25 GMT</pubDate>
        </item>`
	item2 string = `        <item>
            <title><![CDATA[@sdjkals/data-lib-kernel]]></title>
            <description><![CDATA[Assets]]></description>
            <link>https://npmjs.com/package/@sdjkals/data-lib-kernel</link>
            <guid isPermaLink="true">https://npmjs.com/package/@sdjkals/data-lib-kernel</guid>
            <dc:creator><![CDATA[sdjkals]]></dc:creator>
            <pubDate>Sun, 21 Dec 2025 10:08:22 GMT</pubDate>
        </item>`
)

func TestCompareItem(t *testing.T) {
	var i1 Item
	err := xml.Unmarshal([]byte(item1), &i1)
	if err != nil {
		t.Errorf("TestCompareItem:%v", err)
	}
	var i2 Item
	err = xml.Unmarshal([]byte(item2), &i2)
	if err != nil {
		t.Errorf("TestCompareItem:%v", err)
	}
	if i1.Is(&i2) {
		t.Error("incorrect comparison - should be different")
	}
	if !i2.Is(&i2) {
		t.Error("incorrect comparison - should be same")

	}
}
