import asyncio
from playwright.async_api import async_playwright
def svg(detail=True):
    speed = '' if not detail else '''
  <g stroke="#efeee8" stroke-width="10" stroke-linecap="round" opacity=".9">
    <path d="M22 112 H52"/><path d="M14 140 H48"/><path d="M26 168 H50"/></g>'''
    checks = '' if not detail else ''.join(
        f'<rect x="{84+i*12}" y="{166+j*12}" width="12" height="12" fill="{"#1d1e21" if (i+j)%2 else "#efeee8"}"/>' for i in range(5) for j in range(2))
    glint = '' if not detail else '<path d="M168 112 Q190 110 204 120" stroke="#ffffff" stroke-width="7" stroke-linecap="round" fill="none" opacity=".85"/>'
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="256" height="256">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#2f3238"/><stop offset="1" stop-color="#1e2024"/></linearGradient>
    <linearGradient id="hel" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ff4b3e"/><stop offset="1" stop-color="#c41d1d"/></linearGradient>
    <linearGradient id="vis" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#3b4bd8"/><stop offset=".55" stop-color="#16193a"/><stop offset="1" stop-color="#0c0d1c"/></linearGradient>
    <clipPath id="tile"><rect x="8" y="8" width="240" height="240" rx="56"/></clipPath>
  </defs>
  <rect x="8" y="8" width="240" height="240" rx="56" fill="url(#bg)"/>
  <g clip-path="url(#tile)">
  <rect x="8" y="206" width="240" height="14" fill="#d42a2a"/>
  {"".join(f'<rect x="{8+i*30+15}" y="206" width="15" height="14" fill="#efeee8"/>' for i in range(8))}
  </g>
  <rect x="8" y="8" width="240" height="240" rx="56" fill="none" stroke="#000" stroke-opacity=".25" stroke-width="2"/>
  {speed}
  <path d="M62 152 C 60 86, 116 54, 160 56 C 206 58, 226 98, 224 140 L 224 168 C 224 184, 212 194, 196 194 L 100 194 C 76 194, 62 178, 62 152 Z" fill="url(#hel)" stroke="#1d1e21" stroke-width="7" stroke-linejoin="round"/>
  <path d="M74 112 C 92 76, 130 62, 164 64" stroke="#efeee8" stroke-width="12" fill="none" stroke-linecap="round"/>
  {checks}
  <path d="M150 100 C 182 96, 220 106, 224 128 L 224 152 L 162 152 C 146 152, 136 142, 136 128 C 136 112, 140 102, 150 100 Z" fill="url(#vis)" stroke="#1d1e21" stroke-width="7" stroke-linejoin="round"/>
  {glint}
</svg>'''
async def main():
    async with async_playwright() as p:
        b=await p.chromium.launch(executable_path="/usr/bin/google-chrome")
        pg=await b.new_page(viewport={"width":256,"height":256},device_scale_factor=1)
        for name,det in (("big",True),("small",False)):
            s=svg(det); open(f"{name}.svg","w").write(s)
            await pg.set_content(f'<html><body style="margin:0;background:transparent">{s}</body></html>')
            await pg.screenshot(path=f"{name}.png",omit_background=True,clip={"x":0,"y":0,"width":256,"height":256})
        await b.close()
asyncio.run(main())
from PIL import Image
big=Image.open("big.png").convert("RGBA"); small=Image.open("small.png").convert("RGBA")
imgs={s:(big if s>=48 else small).resize((s,s),Image.LANCZOS) for s in (16,20,24,32,40,48,64,128,256)}
imgs[256].save("simlauncher.ico",sizes=[(s,s) for s in imgs],append_images=[imgs[s] for s in imgs if s!=256])
prev=Image.new("RGBA",(256+128+64+48+32+16+60,256),(240,240,240,255)); x=0
for s in (256,128,64,48,32,16):
    prev.paste(imgs[s],(x,256-s),imgs[s]); x+=s+10
dark=prev.copy(); 
prev.save("logo_preview.png")
print("ok")
