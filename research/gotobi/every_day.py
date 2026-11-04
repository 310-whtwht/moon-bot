import sys, numpy as np, pandas as pd, calendar, datetime as dt
def gotobi_days(years):
    out=set()
    for y in years:
        for m in range(1,13):
            last=calendar.monthrange(y,m)[1]
            for d in (5,10,15,20,25,last):
                x=dt.date(y,m,d)
                while x.weekday()>=5: x-=dt.timedelta(days=1)
                out.add(x)
    return out
G=gotobi_days(range(2023,2027))
for path in sys.argv[1:]:
    c=pd.read_csv(path,parse_dates=["time"]).set_index("time")
    j=c.index+pd.Timedelta(hours=9)
    ent=c[(j.hour==9)&(j.minute==55)]
    rows=[]
    for t,r in ent.iterrows():
        tx=t+pd.Timedelta(minutes=25)
        if tx not in c.index: continue
        pnl=(r.bid_open-0.005)-(c.ask_open[tx]+0.005)
        d=(t+pd.Timedelta(hours=9)).date()
        rows.append((d, d in G, d.weekday(), pnl*100))  # 銭
    df=pd.DataFrame(rows,columns=["d","g","wd","sen"])
    pf=lambda x: x[x>0].sum()/-x[x<0].sum()
    print(path.split("/")[-1])
    for name,m in (("ゴトー日",df.g),("ゴトー日以外",~df.g)):
        x=df[m]; h=len(x)//2
        print(f"  {name}: {len(x)}日 平均 {x.sen.mean():+.2f}銭 PF {pf(x.sen):.2f}（前半 {pf(x.sen.iloc[:h]):.2f} / 後半 {pf(x.sen.iloc[h:]):.2f}）")
    x=df[~df.g]
    print("   ゴトー日以外の曜日別 PF:", {["月","火","水","木","金"][w]: round(pf(g.sen),2) for w,g in x.groupby("wd")})
