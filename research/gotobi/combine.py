import json, sys, numpy as np, pandas as pd
S=sys.argv[1]
rows=[]
for sym in ["USD_JPY","GBP_JPY","EUR_JPY","AUD_JPY"]:
    d=json.load(open(f"{S}/b/{sym}.json"))[0]
    for t in d["trades"]:
        rows.append(dict(sym=sym, t=pd.Timestamp(t["entry_time"]), pnl=t["pnl"], px=t["entry_price"]))
df=pd.DataFrame(rows); df["day"]=(df.t+pd.Timedelta(hours=9)).dt.date
# 1万通貨あたりに直す（pnl は100通貨）
df["pnl"]*=100
piv=df.pivot_table(index="day",columns="sym",values="pnl",aggfunc="sum")
print("同じ日の損益の相関\n", piv.corr().round(2))
def stats(name, s):
    s=s.sort_index(); eq=s.cumsum(); dd=(eq.cummax()-eq).max()
    years=(pd.Timestamp(s.index[-1])-pd.Timestamp(s.index[0])).days/365.25
    m=s.groupby(pd.to_datetime(pd.Series(s.index,index=s.index)).dt.to_period("M")).sum()
    half=len(s)//2
    pf=lambda x: x[x>0].sum()/-x[x<0].sum()
    print(f"{name}: {len(s)}日 年あたり {s.sum()/years:+,.0f}円 最大DD {dd:,.0f}円 年利益÷DD {s.sum()/years/dd:.1f} "
          f"黒字の月 {(m>0).sum()}/{len(m)} 最悪の月 {m.min():+,.0f}円 PF 前半 {pf(s.iloc[:half]):.2f} 後半 {pf(s.iloc[half:]):.2f}")
    return dd, s.sum()/years
print("\n各1万通貨")
for sym in piv.columns: stats(sym, piv[sym].dropna())
stats("USD+GBP+EUR", piv[["USD_JPY","GBP_JPY","EUR_JPY"]].sum(axis=1, min_count=1).dropna())
dd, yr = stats("4銘柄", piv.sum(axis=1, min_count=1).dropna())
px=df.groupby("sym").px.mean(); print("\n平均レート", px.round(1).to_dict())
