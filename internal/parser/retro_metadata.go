package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RetroLabel は見出しの年月と番号を表す。ここでは対象期間を確定しない。
type RetroLabel struct {
	Month    string
	Sequence int32
}

// RetroPeriod は日本時間の暦日による対象期間を表す。開始日・終了日の両方を含む。
type RetroPeriod struct {
	StartDate string
	EndDate   string
}

// SprintPolicy は適用開始月以降の見出しに、月曜〜日曜のsprint規約を適用する。
// ゼロ値では期間計算を無効にし、過去の通し番号から期間を推測しない。
type SprintPolicy struct {
	fromMonth string
}

// NewSprintPolicy は適用開始月を検証する。空文字なら期間計算を無効にする。
func NewSprintPolicy(fromMonth string) (SprintPolicy, error) {
	if fromMonth == "" {
		return SprintPolicy{}, nil
	}
	month, err := time.Parse("2006-01", fromMonth)
	if err != nil || month.Year() < 1 || month.Format("2006-01") != fromMonth {
		return SprintPolicy{}, fmt.Errorf("sprint start month must be YYYY-MM with year 0001 through 9999")
	}
	return SprintPolicy{fromMonth: fromMonth}, nil
}

// Period は、見出しの年月の第N月曜日から日曜日までを返す。
// 規約が無効・適用対象外・見出しが不正な場合は、期間未設定を表すnilを返す。
func (p SprintPolicy) Period(label *RetroLabel) *RetroPeriod {
	if p.fromMonth == "" || label == nil || label.Month < p.fromMonth || label.Sequence < 1 || label.Sequence > 5 {
		return nil
	}
	month, err := time.Parse("2006-01", label.Month)
	if err != nil || month.Year() < 1 || month.Format("2006-01") != label.Month {
		return nil
	}
	// 月の1日から最初の月曜日までの日数（0〜6日）を求める。
	// 暦日の計算なので、取り込み日時やUTCから日本時間への変換は使わない。
	firstMondayOffset := (int(time.Monday) - int(month.Weekday()) + 7) % 7
	// 第N月曜日は、第1月曜日のN-1週間後。そこから6日後の日曜日までを対象にする。
	start := month.AddDate(0, 0, firstMondayOffset+7*int(label.Sequence-1))
	end := start.AddDate(0, 0, 6)
	// 存在しない第5月曜日や、保存可能な年の上限を超える期間は採用しない。
	if start.Month() != month.Month() || end.Year() > 9999 {
		return nil
	}
	return &RetroPeriod{StartDate: start.Format(time.DateOnly), EndDate: end.Format(time.DateOnly)}
}

var retroHeading = regexp.MustCompile(`^# ([0-9]{4})(0[1-9]|1[0-2])-([1-9][0-9]*)$`)

// ExtractRetroLabel は最初の非空行にある「# YYYYMM-N」から年月と番号を読み取る。
// 形式が合わない場合はnilを返し、ファイル名や本文中の日付では補わない。
func ExtractRetroLabel(content []byte) *RetroLabel {
	text := strings.TrimPrefix(string(content), "\ufeff")
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// 行頭の字下げは残し、コードブロックを見出しとして扱わない。
		match := retroHeading.FindStringSubmatch(strings.TrimRight(line, " \t\r"))
		if match == nil || match[1] == "0000" {
			return nil
		}
		// match[1]は年、match[2]は月、match[3]は末尾の番号。
		// 番号を10進数として読み、Protobufのint32に収まることを確認する。
		sequence, err := strconv.ParseInt(match[3], 10, 32)
		if err != nil {
			return nil
		}
		// 例:「# 202611-2」→ Month: "2026-11", Sequence: 2。期間の計算はPeriodが行う。
		return &RetroLabel{Month: match[1] + "-" + match[2], Sequence: int32(sequence)}
	}
	return nil
}
