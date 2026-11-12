import asyncio, os
from playwright.async_api import async_playwright
W='#efeee8'
def car(y=0):  # silhouette GT vue de profil
    return f'''<g transform="translate(26 6) scale(0.82)"><path d="M40 {150+y} q6-26 40-30 l40-26 q14-8 34-8 h34 q22 0 40 18 l18 16 q30 4 38 22 v14 h-244z" fill="{W}"/>
<circle cx="88" cy="{164+y}" r="17" fill="#222" stroke="{W}" stroke-width="7"/><circle cx="208" cy="{164+y}" r="17" fill="#222" stroke="{W}" stroke-width="7"/></g>'''
P={
 "ams2":("AMS2","#1f8a4c",f'''<circle cx="144" cy="118" r="62" fill="none" stroke="{W}" stroke-width="14"/>
   <path d="M86 118 h116 M144 118 v60" stroke="{W}" stroke-width="14"/><circle cx="144" cy="118" r="16" fill="{W}"/>'''),
 "acc":("ACC","#1f5fbf",car(-20)),
 "ac":("AC","#b3261e",f'''<path d="M112 196 C 72 160, 206 136, 150 104 S 126 62, 170 50" fill="none" stroke="{W}" stroke-width="30" stroke-linecap="round"/>
   <path d="M112 196 C 72 160, 206 136, 150 104 S 126 62, 170 50" fill="none" stroke="#b3261e" stroke-width="4" stroke-dasharray="12 12"/>'''),
 "acevo":("AC EVO","#d9480f",f'''<path d="M64 150 A80 80 0 1 1 224 150" fill="none" stroke="{W}" stroke-width="14" stroke-linecap="round"/>
   <path d="M144 150 L196 92" stroke="{W}" stroke-width="10" stroke-linecap="round"/><circle cx="144" cy="150" r="13" fill="{W}"/>
   <path d="M80 100 l12 8 M144 70 v14 M208 100 l-12 8" stroke="{W}" stroke-width="7" stroke-linecap="round"/>'''),
 "acrally":("AC RALLY","#5a7d2a",f'''<path d="M30 170 L100 70 L140 120 L170 85 L258 170 Z" fill="{W}"/>
   <path d="M100 70 L118 94 L104 92 L92 100 Z" fill="#5a7d2a"/>
   <path d="M80 200 Q144 160 210 200" fill="none" stroke="{W}" stroke-width="10" stroke-dasharray="4 10" stroke-linecap="round"/>'''),
 "lmu":("LMU","#0b2f6b",f'''<circle cx="144" cy="112" r="68" fill="none" stroke="{W}" stroke-width="12"/>
   <text x="144" y="134" text-anchor="middle" font-family="DejaVu Sans Condensed" font-weight="700" font-size="62" fill="{W}">24</text>
   <path d="M144 44 v12 M212 112 h-12 M144 180 v-12 M76 112 h12" stroke="{W}" stroke-width="8"/>'''),
 "raceroom":("RACEROOM","#6b2fb3","".join(f'<rect x="{78+(i%5)*26}" y="{50+(i//5)*26}" width="26" height="26" fill="{W if (i%5+i//5)%2==0 else "#6b2fb3"}"/>' for i in range(25))+f'<rect x="78" y="50" width="130" height="130" fill="none" stroke="{W}" stroke-width="4"/><path d="M70 44 V214" stroke="{W}" stroke-width="10" stroke-linecap="round"/>'),
 "beamng":("BEAMNG","#e07a1f",car(-20)+f'''<path d="M232 70 l8 22 22-8 -14 18 18 12 -24 2 4 22 -16-16 -14 18 -2-24 -22 4 18-14 -14-18 22 8z" fill="#ffd43b" stroke="#222" stroke-width="3"/>'''),
}
def svg(gid):
    name,col,pic=P[gid]
    fs=44 if len(name)<=6 else 34
    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="288" height="288" viewBox="0 0 288 288">
<rect width="288" height="288" rx="0" fill="{col}"/>
<g>{pic}</g>
<rect x="0" y="222" width="288" height="66" fill="#1d1e21"/>
<g>{"".join(f'<rect x="{i*24}" y="222" width="12" height="6" fill="{"#d42a2a" if i%2==0 else W}"/><rect x="{i*24+12}" y="222" width="12" height="6" fill="{W if i%2==0 else "#d42a2a"}"/>' for i in range(12))}</g>
<text x="144" y="{270 if fs==44 else 266}" text-anchor="middle" font-family="DejaVu Sans Condensed" font-weight="700" font-size="{fs}" fill="{W}" letter-spacing="1">{name}</text>
</svg>'''
async def main():
    os.makedirs('out',exist_ok=True)
    async with async_playwright() as p:
        b=await p.chromium.launch(executable_path="/usr/bin/google-chrome")
        pg=await b.new_page(viewport={"width":288,"height":288})
        for gid in P:
            open(f'out/{gid}.svg','w').write(svg(gid))
            await pg.set_content(f'<html><body style="margin:0">{svg(gid)}</body></html>')
            await pg.screenshot(path=f'out/{gid}.png',clip={"x":0,"y":0,"width":288,"height":288})
        await b.close()
asyncio.run(main())
