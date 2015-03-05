<!DOCTYPE html>
<html>
<head>
  <meta charset="GB2312">
  <meta http-equiv="Pragma" content="no-cache">
  
  <title>设备{{.ID}}实时视频</title>

  <link href="/static/js/hls/videojs-hls/video.js/dist/video-js/video-js.css" rel="stylesheet">
  
  <!-- video.js -->
  <script src="/static/js/hls/videojs-hls/video.js/dist/video-js/video.js"></script>

  <!-- Media Sources plugin -->
  <script src="/static/js/hls/videojs-hls/videojs-contrib-media-sources/videojs-media-sources.js"></script>

  <!-- HLS plugin -->
  <script src="/static/js/hls/videojs-hls/src/videojs-hls.js"></script>

  <!-- segment handling -->
  <script src="/static/js/hls/videojs-hls/src/xhr.js"></script>
  <script src="/static/js/hls/videojs-hls/src/flv-tag.js"></script>
  <script src="/static/js/hls/videojs-hls/src/exp-golomb.js"></script>
  <script src="/static/js/hls/videojs-hls/src/h264-stream.js"></script>
  <script src="/static/js/hls/videojs-hls/src/aac-stream.js"></script>
  <script src="/static/js/hls/videojs-hls/src/segment-parser.js"></script>

  <!-- m3u8 handling -->
  <script src="/static/js/hls/videojs-hls/src/stream.js"></script>
  <script src="/static/js/hls/videojs-hls/src/m3u8/m3u8-parser.js"></script>
  <script src="/static/js/hls/videojs-hls/src/playlist-loader.js"></script>

  <script src="/static/js/hls/videojs-hls/pkcs7/dist/pkcs7.unpad.js"></script>
  <script src="/static/js/hls/videojs-hls/src/decrypter.js"></script>

  <script src="/static/js/hls/videojs-hls/src/bin-utils.js"></script>
  
  <!-- example MPEG2-TS segments -->
  <!-- bipbop -->
  <!-- <script src="test/tsSegment.js"></script> -->
  <!-- bunnies -->
  <!--<script src="test/tsSegment-bc.js"></script>-->

  <style>
    body {
      font-family: Arial, sans-serif;
      margin: 20px;
    }
    .info {
      background-color: #eee;
      border: thin solid #333;
      border-radius: 3px;
      padding: 0 5px;
      margin: 20px 0;
    }
  </style>

</head>
<body onunload="location.href='www.baidu.com'">
 
  <video id="video"
         class="video-js vjs-default-skin"
         height="300"
         width="600"
		 loop="loop"
		 controls autoplay="autoplay"
		 >
    <source
       src="/static/hls/{{.ID}}.m3u8"
       type="application/x-mpegURL">
  </video>
  <script>
  
    
    // initialize the player
    var player = videojs('video');

	// 错误事件处理逻辑
	player.one("error",function(){
				console.log("Hello World,error");
				player.error("数据正在准备中,正在刷新!");
				setTimeout("self.location.reload();",3000);}
	);
		


  </script>
</body>
</html>
