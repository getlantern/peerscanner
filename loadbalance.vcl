backend F_sp1 {
    .connect_timeout = 10s;
    .port = "80";
    .host = "128.199.176.82";
    .first_byte_timeout = 30s;
    .saintmode_threshold = 200000;
    .max_connections = 20000;
    .between_bytes_timeout = 80s;
    .share_key = "11yqoXJrAAGxPiC07v3q9Z";
  
      
    .probe = {
        .request = "HEAD / HTTP/1.1" "Host: getiantem.org" "Connection: close""User-Agent: Varnish/fastly (healthcheck)";
        .threshold = 3;
        .window = 5;
        .timeout = 5s;
        .initial = 4;
        .expected_response = 200;
        .interval = 15s;
      }
}
backend F_sp3 {
    .connect_timeout = 10s;
    .port = "80";
    .host = "128.199.140.101";
    .first_byte_timeout = 30s;
    .saintmode_threshold = 200000;
    .max_connections = 20000;
    .between_bytes_timeout = 80s;
    .share_key = "11yqoXJrAAGxPiC07v3q9Z";
  
      
    .probe = {
        .request = "HEAD / HTTP/1.1" "Host: getiantem.org" "Connection: close""User-Agent: Varnish/fastly (healthcheck)";
        .threshold = 3;
        .window = 5;
        .timeout = 5s;
        .initial = 4;
        .expected_response = 200;
        .interval = 15s;
      }
}
backend F_sp2 {
    .connect_timeout = 10s;
    .port = "80";
    .host = "128.199.178.148";
    .first_byte_timeout = 30s;
    .saintmode_threshold = 200000;
    .max_connections = 20000;
    .between_bytes_timeout = 80s;
    .share_key = "11yqoXJrAAGxPiC07v3q9Z";
  
      
    .probe = {
        .request = "HEAD / HTTP/1.1" "Host: getiantem.org" "Connection: close""User-Agent: Varnish/fastly (healthcheck)";
        .threshold = 3;
        .window = 5;
        .timeout = 5s;
        .initial = 4;
        .expected_response = 200;
        .interval = 15s;
      }
}
backend F_sp4 {
    .connect_timeout = 10s;
    .port = "80";
    .host = "128.199.140.103";
    .first_byte_timeout = 30s;
    .saintmode_threshold = 200000;
    .max_connections = 20000;
    .between_bytes_timeout = 80s;
    .share_key = "11yqoXJrAAGxPiC07v3q9Z";
  
      
    .probe = {
        .request = "HEAD / HTTP/1.1" "Host: getiantem.org" "Connection: close""User-Agent: Varnish/fastly (healthcheck)";
        .threshold = 3;
        .window = 5;
        .timeout = 5s;
        .initial = 4;
        .expected_response = 200;
        .interval = 15s;
      }
}

director FallbackAutoDirector random {
   .quorum = 1%;
   .retries = 10;
   {
    .backend = F_sp1;
    .weight  = 100;
   }{
    .backend = F_sp2;
    .weight  = 100;
   }{
    .backend = F_sp3;
    .weight  = 100;
   }{
    .backend = F_sp4;
    .weight  = 100;
   }
}

sub vcl_recv {
  set req.backend = FallbackAutoDirector;

  if( req.http.host == "sp1.getiantem.org" ) {
    set req.backend = F_sp1;
  }
  if( req.http.host == "sp2.getiantem.org" ) {
    set req.backend = F_sp2;
  }
  if( req.http.host == "sp3.getiantem.org" ) {
    set req.backend = F_sp3;
  }
  if( req.http.host == "sp4.getiantem.org" ) {
    set req.backend = F_sp4;
  }
  #FASTLY recv
}