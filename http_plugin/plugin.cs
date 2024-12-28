using System;
using System.Text;
using System.Net;
using System.Collections.Concurrent;
using System.Threading;
using Newtonsoft.Json.Linq;
using Newtonsoft.Json;
using BilibiliDM_PluginFramework;

namespace blive_dm_http
{
    public class Class1 : BilibiliDM_PluginFramework.DMPlugin
    {
        private readonly HttpListener _httpListener = new HttpListener();
        private Thread _httpListenerThread;
        private readonly BlockingCollection<JToken> _messageQueue = new BlockingCollection<JToken>();

        private void HandleIncomingConnections()
        {
            while (_httpListener.IsListening)
            {
                try
                {

                    var context = _httpListener.GetContext();
                    var response = context.Response;
                    response.ContentType = "application/json";
                    var messages = new JArray();

                    if (_messageQueue.TryTake(out JToken incoming, 1000))
                    {
                        messages.Add(incoming);
                        while (_messageQueue.Count > 0 && messages.Count < 100)
                        {
                            messages.Add(_messageQueue.Take());
                        }

                    }
                    string responseString = JsonConvert.SerializeObject(messages, Formatting.None);
                    var buffer = Encoding.UTF8.GetBytes(responseString);
                    response.ContentLength64 = buffer.Length;
                    response.OutputStream.Write(buffer, 0, buffer.Length);
                    response.OutputStream.Close();
                }
                catch (Exception e)
                {
                    this.Log(e.Message);
                    break;
                }
            }
        }

        public Class1()
        {
            this.Connected += Class1_Connected;
            this.Disconnected += Class1_Disconnected;
            this.ReceivedDanmaku += Class1_ReceivedDanmaku;
            this.ReceivedRoomCount += Class1_ReceivedRoomCount;
            this.PluginAuth = "Hubert Zhang";
            this.PluginName = "弹幕转 HTTP 服务器";
            this.PluginCont = "hubert_zhang@icloud.com";
            this.PluginVer = "v0.0.1";
            this.PluginDesc = "提供 HTTP 接口，访问 127.0.0.1:9876/room/-/msgs 可持续获取弹幕原始数据";
        }

        private void StartServer()
        {
            if (_httpListener.IsListening)
            {
                return;
            }
            _httpListener.Prefixes.Add("http://127.0.0.1:9876/");
            _httpListener.Start();
            _httpListenerThread = new Thread(HandleIncomingConnections);
            _httpListenerThread.Start();

            this.Log("HttpServerPlugin Started!");
            this.AddDM("HttpServerPlugin Started!");
        }

        private void StopServer()
        {
            if (_httpListener.IsListening)
            {
                _httpListener.Stop();
                _httpListener.Close();
                _httpListenerThread.Abort();
            }
        }

        public override void Inited()
        {
            base.Inited();
            if (this.Status)
            {
                StartServer();
            }
            return;
        }

        public override void DeInit()
        {
            base.DeInit();
            StopServer();
        }


        private void Class1_ReceivedRoomCount(object sender, BilibiliDM_PluginFramework.ReceivedRoomCountArgs e)
        {
            return;
        }

        private void Class1_ReceivedDanmaku(object sender, BilibiliDM_PluginFramework.ReceivedDanmakuArgs e)
        {
            switch (e.Danmaku.MsgType) {
                case MsgTypeEnum.Unknown:
                    this.Log(e.Danmaku.RawData);
                    break;
                case MsgTypeEnum.GiftSend:
                case MsgTypeEnum.GiftTop:
                case MsgTypeEnum.GuardBuy:
                case MsgTypeEnum.SuperChat:
                default:
                    break;
            }
            _messageQueue.Add(e.Danmaku.RawDataJToken);
            return;
        }

        private void Class1_Disconnected(object sender, BilibiliDM_PluginFramework.DisconnectEvtArgs e)
        {
            return;
        }

        private void Class1_Connected(object sender, BilibiliDM_PluginFramework.ConnectedEvtArgs e)
        {
            return;
        }

        public override void Stop()
        {
            base.Stop();
            StopServer();
        }

        public override void Start()
        {
            base.Start();
            StartServer();
        }
    }
}
