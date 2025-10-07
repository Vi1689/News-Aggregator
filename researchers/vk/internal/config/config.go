package config

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type ResearcherConfig struct {
	Channel_limit      int
	Post_limit         int
	Comment_limit      int
	Media_limit        int
	Preferred_channels string
	Research_period    int
}

type researcherConfigXML struct {
	XMLName xml.Name `xml:"config"`
	//Text    string   `xml:",chardata"`
	Source []struct {
		//Text        string `xml:",chardata"`
		ChannelLimit string `xml:"channel_limit"`
		Name         string `xml:"name,attr"`
		PostLimit    string `xml:"post_limit"`
		CommentLimit string `xml:"comment_limit"`
		MediaLimit   string `xml:"media_limit"`

		PreferredChannels string `xml:"preferred_channels"`
		ResearchPeriod    string `xml:"research_period"`
	} `xml:"source"`
}

func ParseConfigFile(data string) (ResearcherConfig, error) {
	var config researcherConfigXML
	var retConf ResearcherConfig
	err := xml.Unmarshal([]byte(data), &config)
	if err != nil {
		return retConf, err
	}

	fmt.Printf("--- Unmarshal ---\n\n")
	for _, param := range config.Source {

		fmt.Printf("name: %s\n", param.Name)
		if param.Name != "Vkontakte" {
			continue
		}
		fmt.Printf("channel_limit: %s\n", param.ChannelLimit)
		fmt.Printf("post_limit: %s\n", param.PostLimit)
		fmt.Printf("comment_limit: %s\n", param.CommentLimit)
		fmt.Printf("media_limit: %s\n", param.MediaLimit)
		fmt.Printf("preferred_channels: %s\n", param.PreferredChannels)
		fmt.Printf("research_period: %s\n", param.ResearchPeriod)
		fmt.Printf("---\n")
		var err error
		retConf.Channel_limit, err = strconv.Atoi(strings.TrimSpace(strings.TrimSpace(param.ChannelLimit)))
		if err != nil {
			return retConf, err
		}
		retConf.Post_limit, err = strconv.Atoi(strings.TrimSpace(param.PostLimit))
		if err != nil {
			return retConf, err
		}
		retConf.Comment_limit, err = strconv.Atoi(strings.TrimSpace(param.CommentLimit))
		if err != nil {
			return retConf, err
		}
		retConf.Media_limit, err = strconv.Atoi(strings.TrimSpace(param.MediaLimit))
		if err != nil {
			return retConf, err
		}
		retConf.Preferred_channels = param.PreferredChannels
		retConf.Research_period, err = strconv.Atoi(strings.TrimSpace(param.ResearchPeriod))
		return retConf, err

	}
	return retConf, errors.New("config.ParseConfigFile: there is no Vkontakte config")
}

func ValidateConfig(conf ResearcherConfig) error {
	fmt.Printf("--- ValidateConfig ---\n\n")
	fmt.Printf("channel_limit: %d\n", conf.Channel_limit)
	fmt.Printf("post_limit: %d\n", conf.Post_limit)
	fmt.Printf("comment_limit: %d\n", conf.Comment_limit)
	fmt.Printf("media_limit: %d\n", conf.Media_limit)
	fmt.Printf("preferred_channels: %v\n", conf.Preferred_channels)
	fmt.Printf("research_period: %d\n", conf.Research_period)
	fmt.Printf("---\n")

	if conf.Channel_limit < 0 {
		return errors.New("config.ValidateConfig: channel_limit must be > 0")
	}
	if conf.Post_limit < 0 {
		return errors.New("config.ValidateConfig: post_limit must be > 0")
	}
	if conf.Comment_limit < 0 {
		return errors.New("config.ValidateConfig: comment_limit must be > 0")
	}
	if conf.Media_limit < 0 {
		return errors.New("config.ValidateConfig: media_limit must be > 0")
	}
	if conf.Research_period < 0 {
		return errors.New("config.ValidateConfig: research_period must be > 0")
	}
	return nil
}
