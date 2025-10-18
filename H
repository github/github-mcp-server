<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>WD Beter - ريلز</title>
<style>
  * { margin:0; padding:0; box-sizing:border-box; }
  body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background:#f4f7f8; color:#333; line-height:1.6; }
  a { text-decoration:none; color:inherit; }
  header { background:#023047; color:white; padding:20px 40px; display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; }
  header .logo { font-size:1.8rem; font-weight:bold; letter-spacing:2px; }
  nav a { margin-left:25px; font-weight:600; transition:color 0.3s; }
  nav a:hover { color:#fb8500; }
  .intro { text-align:center; padding:40px 20px; background:linear-gradient(90deg, #023047, #fb8500); color:white; border-radius:10px; margin:20px; }
  .intro h2 { font-size:2rem; margin-bottom:10px; }
  .intro p { font-size:1.1rem; }
  .reels { display:grid; grid-template-columns:repeat(auto-fit,minmax(280px,1fr)); gap:20px; padding:40px; }
  .reel { background:white; border-radius:8px; box-shadow:0 2px 8px rgba(0,0,0,0.1); overflow:hidden; transition:transform 0.3s; }
  .reel:hover { transform:translateY(-5px); }
  .reel iframe { width:100%; height:400px; border:none; }
  .reel-title { padding:10px; font-weight:bold; text-align:center; color:#023047; }
  footer { background:#023047; color:white; text-align:center; padding:20px; margin-top:40px; }
  @media(max-width:600px){ header{flex-direction:column; gap:10px;} nav a{margin-left:10px;} .reel iframe{height:300px;} }
</style>
</head>
<body>

<header>
  <div class="logo">WD Beter</div>
  <nav>
    <a href="#intro">الرئيسية</a>
    <a href="#reels">ريلز</a>
    <a href="#contact">تواصل</a>
  </nav>
</header>

<section id="intro" class="intro">
  <h2>WD Beter</h2>
  <p>صانع محتوى كوميدي وترندات على TikTok وInstagram، يجمع بين التحشيش السوداني والمواقف اليومية بأسلوب فريد ومضحك.</p>
</section>

<section id="reels" class="reels">
  <div class="reel">
    <iframe src="https://www.tiktok.com/embed/7429301234567890123" allowfullscreen></iframe>
    <div class="reel-title">ريل كوميدي مضحك</div>
  </div>
  <div class="reel">
    <iframe src="https://www.tiktok.com/embed/7429309876543210987" allowfullscreen></iframe>
    <div class="reel-title">تحدي ترند</div>
  </div>
  <div class="reel">
    <iframe src="https://www.tiktok.com/embed/7429312345678901234" allowfullscreen></iframe>
    <div class="reel-title">موقف يومي مضحك</div>
  </div>
</section>

<footer>
  &copy; 2025 WD Beter. جميع الحقوق محفوظة.
</footer>

</body>
</html>
