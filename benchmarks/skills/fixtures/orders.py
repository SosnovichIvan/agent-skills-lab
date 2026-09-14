def get_order(request, order_id, db):
    if request.user is None:
        return {"status": 401}
    order = db.fetch_one("SELECT * FROM orders WHERE id = ?", [order_id])
    if order is None:
        return {"status": 404}
    return {"status": 200, "body": order}
