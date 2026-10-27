# /// script
# requires-python = ">=3.11"
# dependencies = ["holidays"]
# ///
"""仲値後の売り（strategies/research/gotobi_v2.star）が見送る日の一覧を、Starlark の形で書き出す。

見送る日（日本時間の日付）:
  - 日本の祝日と、銀行の休業日（12/31・1/2・1/3）… 仲値の公示がない
  - 米国の雇用統計・CPI・FOMC がある取引日（6:00 JST 区切り）… research/events/us_jp_events.csv から

使い方:
  uv run research/events/skip_days.py 2023 2027
出力を gotobi_v2.star の SKIP_DAYS に貼る。予定表を足したら、作り直す。
"""

import csv
import pathlib
import sys
from datetime import date, datetime, timedelta, timezone

import holidays

JST = timezone(timedelta(hours=9))


def main():
    first, last = int(sys.argv[1]), int(sys.argv[2])
    days = set()
    for d in holidays.Japan(years=range(first, last + 1)):
        days.add(d)
    for y in range(first, last + 1):
        days.update({date(y, 12, 31), date(y, 1, 2), date(y, 1, 3)})
    events = pathlib.Path(__file__).with_name("us_jp_events.csv")
    for row in csv.DictReader(open(events)):
        if row["event"] == "boj":
            continue
        t = datetime.fromisoformat(row["time"].replace("Z", "+00:00")).astimezone(JST)
        days.add((t - timedelta(hours=6)).date())  # その発表を含む取引日の、日本時間の日付
    days = sorted(d for d in days if first <= d.year <= last)
    nums = [d.year * 10000 + d.month * 100 + d.day for d in days]
    print(f"# {first}〜{last}年、{len(nums)}日（research/events/skip_days.py で作成）")
    print("SKIP_DAYS = [")
    for i in range(0, len(nums), 8):
        print("    " + ", ".join(str(n) for n in nums[i:i + 8]) + ",")
    print("]")


if __name__ == "__main__":
    main()
