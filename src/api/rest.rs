use axum::{
    body::Bytes,
    extract::MatchedPath,
    http::{HeaderMap, Request, StatusCode},
    response::Response,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::time::Duration;
use tower_http::{classify::ServerErrorsFailureClass, trace::TraceLayer};
use tracing::{info_span, Span};

#[derive(Debug, Serialize, Deserialize)]
pub struct HttpResponse<D> {
    status: u16,
    data: D,
}

async fn healthz() -> Json<HttpResponse<String>> {
    let response = HttpResponse {
        status: StatusCode::OK.into(),
        data: "ok".to_string(),
    };

    Json(response)
}

async fn readyz() -> Json<HttpResponse<String>> {
    let response = HttpResponse {
        status: StatusCode::OK.into(),
        data: "ok".to_string(),
    };

    Json(response)
}

pub fn define_api(router: Router) -> Router {
    router
        .route("/healthz", get(healthz))
        .route("/readyz", get(readyz))
}

pub fn define_logging_layer(router: Router) -> Router {
    router.layer(
        TraceLayer::new_for_http()
            .make_span_with(|request: &Request<_>| {
                // Log the matched route's path (with placeholders not filled in).
                // Use request.uri() or OriginalUri if you want the real path.
                let matched_path = request
                    .extensions()
                    .get::<MatchedPath>()
                    .map(MatchedPath::as_str);

                info_span!(
                    "http_request",
                    method = ?request.method(),
                    matched_path,
                )
            })
            .on_request(|_request: &Request<_>, _span: &Span| {
                // You can use `_span.record("some_other_field", value)` in one of these
                // closures to attach a value to the initially empty field in the info_span
                // created above.
            })
            .on_response(|_response: &Response, _latency: Duration, _span: &Span| {
                // ...
            })
            .on_body_chunk(|_chunk: &Bytes, _latency: Duration, _span: &Span| {
                // ...
            })
            .on_eos(
                |_trailers: Option<&HeaderMap>, _stream_duration: Duration, _span: &Span| {
                    // ...
                },
            )
            .on_failure(
                |_error: ServerErrorsFailureClass, _latency: Duration, _span: &Span| {
                    // ...
                },
            ),
    )
}
