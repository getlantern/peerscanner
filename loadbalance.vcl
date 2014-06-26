sub vcl_recv {
  set req.backend = PeerAutoDirector;

  #FASTLY recv
}