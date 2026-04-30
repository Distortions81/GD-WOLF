package wl6

type VariantSpec struct {
	Name               string
	Ext                string
	StartPicsChunk     int
	StartMusicChunk    int
	MusicCount         int
	IntroSong          int
	MenuSong           int
	TitlePicChunk      int
	GetPsychedPicChunk int
	EpisodeCount       int
	OptionsPicChunk    int
	Cursor1PicChunk    int
	Cursor2PicChunk    int
	MouseBackPicChunk  int
	BabyModePicChunk   int
	EasyPicChunk       int
	NormalPicChunk     int
	HardPicChunk       int
	ControlPicChunk    int
	Episode1PicChunk   int
	LevelPicChunk      int
	PausedPicChunk     int
	StatusBarPicChunk  int
	KnifePicChunk      int
	NoKeyPicChunk      int
	GoldKeyPicChunk    int
	SilverKeyPicChunk  int
	NumberBlankChunk   int
	NumberZeroChunk    int
	Face1APicChunk     int
	Face8APicChunk     int
}

var (
	variantWL6 = VariantSpec{
		Name:               "registered WL6",
		Ext:                "WL6",
		StartPicsChunk:     3,
		StartMusicChunk:    261,
		MusicCount:         27,
		IntroSong:          7,
		MenuSong:           14,
		TitlePicChunk:      87,
		GetPsychedPicChunk: 134,
		EpisodeCount:       6,
		OptionsPicChunk:    10,
		Cursor1PicChunk:    11,
		Cursor2PicChunk:    12,
		MouseBackPicChunk:  18,
		BabyModePicChunk:   19,
		EasyPicChunk:       20,
		NormalPicChunk:     21,
		HardPicChunk:       22,
		ControlPicChunk:    26,
		Episode1PicChunk:   30,
		LevelPicChunk:      38,
		PausedPicChunk:     133,
		StatusBarPicChunk:  86,
		KnifePicChunk:      91,
		NoKeyPicChunk:      95,
		GoldKeyPicChunk:    96,
		SilverKeyPicChunk:  97,
		NumberBlankChunk:   98,
		NumberZeroChunk:    99,
		Face1APicChunk:     109,
		Face8APicChunk:     130,
	}
	variantWL1 = VariantSpec{
		Name:               "shareware WL1",
		Ext:                "WL1",
		StartPicsChunk:     3,
		StartMusicChunk:    261,
		MusicCount:         27,
		IntroSong:          7,
		MenuSong:           14,
		TitlePicChunk:      99,
		GetPsychedPicChunk: 146,
		EpisodeCount:       1,
		OptionsPicChunk:    22,
		Cursor1PicChunk:    23,
		Cursor2PicChunk:    24,
		MouseBackPicChunk:  30,
		BabyModePicChunk:   31,
		EasyPicChunk:       32,
		NormalPicChunk:     33,
		HardPicChunk:       34,
		ControlPicChunk:    38,
		Episode1PicChunk:   42,
		LevelPicChunk:      50,
		PausedPicChunk:     145,
		StatusBarPicChunk:  98,
		KnifePicChunk:      103,
		NoKeyPicChunk:      107,
		GoldKeyPicChunk:    108,
		SilverKeyPicChunk:  109,
		NumberBlankChunk:   110,
		NumberZeroChunk:    111,
		Face1APicChunk:     121,
		Face8APicChunk:     142,
	}
	supportedVariants = []VariantSpec{variantWL6, variantWL1}
)
