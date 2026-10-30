import sys, numpy as np, pandas as pd
sys.path.insert(0,"research/longterm"); sys.path.insert(0,"research/cot")
from donchian_carry import monthly_lagged, series, pf
from cot_filter import load_cot, position_at
f,cp=sys.argv[1:3]
price=series(f,"DEXJPUS"); price=price[price.index>="1985-07-01"]
d=(monthly_lagged(series(f,"FEDFUNDS"),price.index)-monthly_lagged(series(f,"IRSTCI01JPM156N"),price.index)).values
carry=d-np.sign(d)*1.0
pct=position_at(load_cot(cp),price.index.values)
p=price.values; dates=price.index
def sim(exit_n,mode):
    # mode: none=COTなし / retry=見送っても翌日以降入り直す（ボットの動き） / block=見送ったら一度チャネル内に戻るまで入らない
    hi=price.rolling(50).max().shift(1).values; lo=price.rolling(exit_n).min().shift(1).values
    pos=0;e=None;tr=[];blocked=False;action=None;eqd=np.zeros(len(p));
    for i in range(1,len(p)):
        eqd[i]=pos*(p[i]/p[i-1]-1)+pos*carry[i]/100*(dates[i]-dates[i-1]).days/365
        if action=="close":
            s=np.nanmean(carry[e:i+1])/100*(dates[i]-dates[e]).days/365
            tr.append((dates[e],(p[i]-p[e])/p[e]-.03/p[e]+s,(dates[i]-dates[e]).days)); pos=0; eqd[i]-=.015/p[i]
        elif action=="open": pos=1;e=i;eqd[i]-=.015/p[i]
        action=None
        if np.isnan(hi[i]) or np.isnan(lo[i]): continue
        if p[i]<=hi[i]: blocked=False
        if pos==1:
            if p[i]<lo[i]: action="close"
        elif p[i]>hi[i] and d[i]>0 and p[i]>0:
            if mode!="none" and pct[i]>66.7:
                if mode=="block": blocked=True
                continue
            if mode=="block" and blocked: continue
            action="open"
    t=pd.DataFrame(tr,columns=["entry","ret","days"]); t=t[t.entry>="1987"]
    q=pd.Series(eqd,index=dates); q=q[q.index>="1987"]; eq=q.cumsum(); dd=(eq.cummax()-eq).max(); yrs=(q.index[-1]-q.index[0]).days/365.25
    m=t.entry.iloc[len(t)//2]
    return f"{len(t)}回 勝率{(t.ret>0).mean():.0%} PF {pf(t.ret):.2f} (前{pf(t.ret[t.entry<m]):.2f}/後{pf(t.ret[t.entry>=m]):.2f}) 年{q.sum()/yrs*100:+.1f}% 時価DD {dd*100:.0f}% 保有率 {(t.days.sum()/(yrs*365.25)):.0%}"
for n in (20,40):
    for mo,l in (("none","COTなし"),("retry","COTあり・入り直す（今のボット）"),("block","COTあり・チャネルに戻るまで入らない")):
        print(n,l,sim(n,mo))
